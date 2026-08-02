package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/rotisserie/eris"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"backend/graph/generated"
	"backend/graph/resolver"
	"backend/internal/auth"
	"backend/internal/cefr"
	"backend/internal/database"
	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/gqlerr"
	"backend/internal/handler/notionsync"
	"backend/internal/handler/ping"
	"backend/internal/loader"
	"backend/internal/logging"
	internalmw "backend/internal/middleware"
	"backend/internal/notion"
	"backend/internal/repository"
	"backend/internal/telemetry"
	"backend/internal/usecase"
)

const defaultShutdownTimeout = 25 * time.Second

// queryBodyLimitBytes caps the request body at ~2 MiB. It is the single source
// of truth for the BodyLimit installed in newRouter; main_test.go references it
// directly so the test and the production limit can never drift.
const queryBodyLimitBytes int64 = 2 * 1024 * 1024

type serverConfig struct {
	shutdownTimeout time.Duration
	// introspectionEnabled is the single source of truth for whether GraphQL
	// introspection (and the Playground UI that depends on it) is served.
	// Resolved from one predicate so the schema-introspection gate and the
	// Playground route gate can never disagree.
	introspectionEnabled bool
}

// serverConfigFromEnv builds a serverConfig from environment variables.
// SHUTDOWN_TIMEOUT accepts any value accepted by time.ParseDuration; invalid
// or non-positive values fall back to defaultShutdownTimeout with a WARN log.
//
// Introspection is fail-safe: enabled only when GRAPHQL_INTROSPECTION is exactly
// "on". Unset or any other value (including "true") keeps the schema hidden so a
// new deploy target cannot leak it by forgetting to set the variable.
func serverConfigFromEnv(logger *slog.Logger) serverConfig {
	shutdownDur := defaultShutdownTimeout
	if v := os.Getenv("SHUTDOWN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			logger.Warn("invalid SHUTDOWN_TIMEOUT, using default",
				"value", v, "err", err, "default", defaultShutdownTimeout.String())
		} else if d <= 0 {
			logger.Warn("non-positive SHUTDOWN_TIMEOUT, using default",
				"value", v, "default", defaultShutdownTimeout.String())
		} else {
			shutdownDur = d
		}
	}
	return serverConfig{
		shutdownTimeout:      shutdownDur,
		introspectionEnabled: os.Getenv("GRAPHQL_INTROSPECTION") == "on",
	}
}

// appRepos bundles every repository instance, each built once over the same
// *gorm.DB. Bundling removes the previous double-construction of the master
// repos inside the notion-sync block and the long positional argument lists.
// gorm is retained so buildResolver can construct the tx-taking usecases without
// threading db through a separate parameter.
type appRepos struct {
	gorm            *gorm.DB
	user            repository.UserRepository
	role            repository.RoleRepository
	cardgroup       repository.CardgroupRepository
	masterCardgroup repository.MasterCardgroupRepository
	card            repository.CardRepository
	masterCard      repository.MasterCardRepository
	userCardFSRS    repository.UserCardFSRSRepository
	swipeRecord     repository.SwipeRecordRepository
	pingRecord      repository.PingRecordRepository
	userRole        repository.UserRoleRepository
	userPreference  repository.UserPreferenceRepository
}

func newAppRepos(db *database.DB, logger *slog.Logger) *appRepos {
	return &appRepos{
		gorm:            db.GORM,
		user:            repository.NewUserRepository(db.GORM),
		role:            repository.NewRoleRepository(db.GORM),
		cardgroup:       repository.NewCardgroupRepository(db.GORM),
		masterCardgroup: repository.NewMasterCardgroupRepository(db.GORM),
		card:            repository.NewCardRepository(db.GORM),
		masterCard:      repository.NewMasterCardRepository(db.GORM),
		userCardFSRS:    repository.NewUserCardFSRSRepository(db.GORM),
		swipeRecord:     repository.NewSwipeRecordRepository(db.GORM),
		pingRecord:      repository.NewPingRecordRepository(db.GORM),
		userRole:        repository.NewUserRoleRepository(db.GORM),
		userPreference:  repository.NewUserPreferenceRepository(db.GORM, logger),
	}
}

// loaderDeps is the subset of repositories the GraphQL loader middleware needs.
type loaderDeps struct {
	user           repository.UserRepository
	role           repository.RoleRepository
	userRole       repository.UserRoleRepository
	cardgroup      repository.CardgroupRepository
	card           repository.CardRepository
	userPreference repository.UserPreferenceRepository
	swipeRecord    repository.SwipeRecordRepository
	userCardFSRS   repository.UserCardFSRSRepository
}

func (r *appRepos) loaderDeps() loaderDeps {
	return loaderDeps{
		user:           r.user,
		role:           r.role,
		userRole:       r.userRole,
		cardgroup:      r.cardgroup,
		card:           r.card,
		userPreference: r.userPreference,
		swipeRecord:    r.swipeRecord,
		userCardFSRS:   r.userCardFSRS,
	}
}

// buildResolver wires every usecase, the ping/notion handlers, and the GraphQL
// resolver from the repository bundle. Extracted from run() so the wiring is
// independently testable, mirroring bootstrapSuperUserPromoter. notionSyncHandler
// is nil when notion sync is disabled.
func buildResolver(
	repos *appRepos,
	authSvc *auth.Service,
	adminGate *usecase.AdminGate,
	logger *slog.Logger,
	notionEnv notionsync.EnvConfig,
	notionSyncDisabled bool,
	pingToken string,
) (*resolver.Resolver, *ping.Handler, *notionsync.Handler, error) {
	masterDeckUC := usecase.NewMasterDeckUsecase(repos.masterCardgroup, repos.masterCard, repos.card, repos.cardgroup, repos.gorm, logger)
	userUC := usecase.NewUserUsecase(repos.gorm, repos.user, repos.userRole, authSvc, logger)
	cardgroupUC := usecase.NewCardgroupUsecase(repos.cardgroup, authSvc, logger)
	learnUC := usecase.NewLearnUsecase(repos.card, repos.cardgroup, repos.userPreference, service.NewOrderingPolicy(), nil, 0, 0, nil, logger)
	swipeUC := usecase.NewSwipeUsecase(repos.gorm, repos.card, repos.cardgroup, repos.swipeRecord, service.NewFSRSScheduler(), repos.userCardFSRS, logger)
	cardImportUC := usecase.NewCardImportUsecase(repos.cardgroup, repos.card, repos.gorm, logger)
	adminUserUC := usecase.NewAdminUser(repos.gorm, repos.user, repos.role, repos.userRole, adminGate, logger)
	adminRoleUC := usecase.NewAdminRole(repos.role, adminGate, logger)
	lastViewedCardgroupUC := usecase.NewLastViewedCardgroup(repos.userPreference, repos.user, logger)
	updateLearnDisplayModeUC := usecase.NewUpdateLearnDisplayMode(repos.userPreference, repos.user, logger)
	updateNewCardRatioUC := usecase.NewUpdateNewCardRatio(repos.userPreference, repos.user, logger)
	pingHandler := ping.New(repos.pingRecord, pingToken)

	var notionSyncHandler *notionsync.Handler
	var cardObserver usecase.CardObserver
	if !notionSyncDisabled {
		var err error
		cardObserver, notionSyncHandler, err = buildNotionIntegration(repos, notionEnv, logger)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	cardUC := usecase.NewCardUsecase(repos.gorm, repos.card, repos.cardgroup, repos.userCardFSRS, cardObserver, logger)
	// Type the word list as the domain.CEFRWordList port so the dependency
	// edge the constructor creates is domain_service -> domain (allowed),
	// rather than attributing the concrete *cefr.WordList type to a
	// cefr -> domain_service edge (which the layer model forbids).
	var cefrWords domain.CEFRWordList = cefr.NewWordList()
	cefrClassifier := service.NewCEFRClassifier(cefrWords)
	cefrUC := usecase.NewCEFRUsecase(cefrClassifier)
	masterCatalogUC := usecase.NewMasterCatalogUsecase(repos.masterCardgroup, masterDeckUC, repos.cardgroup, adminGate, logger)
	masterCardUC := usecase.NewMasterCardUsecase(repos.gorm, repos.masterCard, repos.masterCardgroup, adminGate, logger)
	statsUC := usecase.NewStats(repos.userCardFSRS, repos.swipeRecord, repos.cardgroup, nil)

	resolvers := resolver.NewResolver(userUC, cardgroupUC, cardUC, swipeUC, authSvc, cardImportUC, adminUserUC, adminRoleUC, lastViewedCardgroupUC, updateLearnDisplayModeUC, updateNewCardRatioUC, learnUC, cefrUC, masterCatalogUC, masterCardUC, statsUC)
	return resolvers, pingHandler, notionSyncHandler, nil
}

// buildNotionIntegration constructs the optional Notion sync infrastructure (the
// fetcher, writer, and card writebacker) plus the sync handler, reading retry
// tuning from the environment. Extracted from buildResolver so the optional
// wiring and its single error path are independently testable, mirroring
// bootstrapSuperUserPromoter. The caller gates this on notionSyncDisabled to
// preserve the disabled-path nil semantics; the only error path is
// notion.RetryConfigFromEnv failing on a malformed env var.
func buildNotionIntegration(
	repos *appRepos,
	notionEnv notionsync.EnvConfig,
	logger *slog.Logger,
) (usecase.CardObserver, *notionsync.Handler, error) {
	retryCfg, err := notion.RetryConfigFromEnv()
	if err != nil {
		return nil, nil, err
	}
	retryCfg.Logger = logger
	notionFetcher := notion.NewFetcher(notionEnv.NotionToken, retryCfg)
	notionWriter := notion.NewWriter(notionEnv.NotionToken, retryCfg)
	cardObserver := notion.NewCardWritebacker(notionWriter, notionEnv.HandlerConfig.PageIDs[0], logger)
	notionSyncUC := usecase.NewMasterNotionSyncUsecase(notionFetcher, repos.masterCardgroup, repos.masterCard, repos.gorm, logger)
	notionSyncHandler := notionsync.New(notionSyncUC, notionEnv.HandlerConfig)
	return cardObserver, notionSyncHandler, nil
}

func newGraphQLServer(r *resolver.Resolver, introspectionEnabled bool) *handler.Server {
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.POST{
		ResponseHeaders: http.Header{
			"Content-Type": []string{"application/graphql-response+json; charset=utf-8"},
		},
	})

	srv.SetRecoverFunc(gqlerr.RecoverFunc)

	srv.Use(extension.FixedComplexityLimit(100))

	// Introspection is fail-safe: opt-in only. The introspectionEnabled flag is
	// resolved once in serverConfigFromEnv from a single predicate
	// (GRAPHQL_INTROSPECTION == "on"), so this schema gate and the Playground
	// route gate in newRouter can never disagree.
	if introspectionEnabled {
		srv.Use(extension.Introspection{})
	}

	srv.Use(otelgqlgen.Middleware())
	srv.Use(extension.AutomaticPersistedQuery{Cache: lru.New[string](100)})

	return srv
}

func newRouter(
	resolvers *resolver.Resolver,
	authMW echo.MiddlewareFunc,
	promoter *auth.SuperUserPromoter,
	ld loaderDeps,
	pingHandler *ping.Handler,
	notionSyncHandler *notionsync.Handler,
	introspectionEnabled bool,
) *echo.Echo {
	e := echo.New()
	e.Use(internalmw.RequestID())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestLogger())
	// Cap the request body at ~2 MiB. gqlgen's transport.POST buffers the entire
	// /query body into memory before parsing, so without this limit an
	// unauthenticated oversized POST is a remote memory-exhaustion vector. The
	// cap leaves headroom over the 1 MiB card-import payload plus JSON overhead.
	// A request with an honest Content-Length header is rejected with 413 before
	// the body is read; a chunked / no-Content-Length request is instead capped
	// during the read by BodyLimit's wrapped limitedReader once the bytes read
	// exceed the cap.
	e.Use(middleware.BodyLimit(queryBodyLimitBytes))

	e.GET("/", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"service": "flamingo-armond-backend",
		})
	})

	// /health answers both GET and HEAD. GET serves Render's deploy/liveness
	// health checks; HEAD serves free-tier keep-warm pings from external uptime
	// monitors that only issue HEAD requests. Echo does not auto-serve HEAD for
	// a GET route (the router resolves HEAD to its own slot with no GET
	// fallback), so a HEAD-only registration gap returns 405; register both
	// methods explicitly. The HEAD response body is dropped by net/http, so the
	// shared handler returns the same 200 payload for both.
	healthz := func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
		})
	}
	e.Match([]string{http.MethodGet, http.MethodHead}, "/health", healthz)

	e.POST("/internal/ping", pingHandler.Handle, pingHandler.RateLimiter())
	if notionSyncHandler != nil {
		e.POST("/internal/notion-sync", notionSyncHandler.Handle)
	}

	gqlSrv := newGraphQLServer(resolvers, introspectionEnabled)
	// Wrap only the GraphQL POST handler with otelhttp so the HTTP layer
	// extracts an incoming traceparent and creates the root HTTP span.
	// /health and /playground are intentionally excluded to reduce noise.
	//
	// WithPropagators: otelhttp.newConfig captures otel.GetTextMapPropagator()
	// eagerly at construction time (config.go:57 in otelhttp@v0.61.0). The
	// default behavior (omitting WithPropagators) does the same eager capture.
	// Therefore newRouter MUST be called after telemetry.Init so that the global
	// propagator is already set when this handler is constructed; otherwise
	// traceparent extraction silently falls back to the no-op propagator.
	otelGQLHandler := otelhttp.NewHandler(
		gqlSrv,
		"graphql.http",
		otelhttp.WithPropagators(otel.GetTextMapPropagator()),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)
	q := e.Group("/query", authMW, promoter.Middleware(), loader.MiddlewareWithUserCardFSRS(ld.user, ld.role, ld.userRole, ld.cardgroup, ld.card, ld.userPreference, ld.swipeRecord, ld.userCardFSRS))
	q.POST("", echo.WrapHandler(otelGQLHandler))

	// Gate the Playground UI on the same single introspectionEnabled flag as the
	// schema introspection gate in newGraphQLServer: the UI is useless without
	// introspection, so a deployment with introspection disabled does not serve
	// it at all rather than serving a non-functional page.
	if introspectionEnabled {
		e.GET("/playground", echo.WrapHandler(playground.Handler("GraphQL", "/query")))
	}

	return e
}

// bootstrapSuperUserPromoter constructs the SuperUserPromoter and emits the
// startup INFO/WARN logs for the super-user bootstrap path. Extracted so its
// branching logic can be unit-tested without spinning up the full run() server.
//
// Returns the promoter and an error only when the admin role lookup fails for
// a non-empty SUPER_USER_EMAILS configuration. A failure to count existing
// admin role-holders is non-fatal: the WARN log records the error_chain and
// the function returns a pass-through promoter.
func bootstrapSuperUserPromoter(
	ctx context.Context,
	logger *slog.Logger,
	authSvc *auth.Service,
	roleRepo repository.RoleRepository,
	userRoleRepo repository.UserRoleRepository,
	emailsEnv string,
) (*auth.SuperUserPromoter, error) {
	superUserEmails := auth.ParseSuperUserSet(emailsEnv)
	if len(superUserEmails) > 0 {
		adminRole, err := roleRepo.FindByName(ctx, domain.AdminRoleName)
		if err != nil {
			return nil, eris.Wrap(err, "run: lookup admin role for super-user bootstrap")
		}
		logger.Info("super-user bootstrap enabled", "email_count", len(superUserEmails))
		return auth.NewSuperUserPromoter(superUserEmails, adminRole.ID, authSvc, userRoleRepo, logger), nil
	}
	// No SUPER_USER_EMAILS configured. Check whether at least one admin
	// already exists in the DB; if not, the operator has no escape hatch
	// and we emit a single-line WARN to make the misconfiguration visible.
	// A failed count query is non-fatal — log the eris chain and continue.
	if adminCount, err := userRoleRepo.CountAdmins(ctx); err != nil {
		logging.LogWarn(ctx, logger, "super-user bootstrap: admin count check failed",
			eris.Wrap(err, "run: count admin users for bootstrap check"))
	} else if adminCount == 0 {
		logger.Warn("super-user bootstrap: no admin configured and no admin role-holder exists",
			"admin_count", adminCount)
	}
	return auth.NewSuperUserPromoter(nil, "", nil, nil, logger), nil
}

func run(ctx context.Context, logger *slog.Logger) error {
	// telemetry.Init must run first: it registers the global TracerProvider and
	// TextMapPropagator via otel.SetTextMapPropagator. newRouter (called below)
	// constructs the otelhttp handler, which captures otel.GetTextMapPropagator()
	// eagerly at construction time. Reversing this order would silently drop
	// traceparent extraction for every incoming request.
	tracerShutdown, err := telemetry.Init(ctx, logger)
	if err != nil {
		return eris.Wrap(err, "run: telemetry init")
	}
	srvCfg := serverConfigFromEnv(logger)

	pingToken := os.Getenv("PING_TOKEN")
	if pingToken == "" {
		return eris.New("run: PING_TOKEN env var is required")
	}

	cfg, err := auth.ConfigFromEnv()
	if err != nil {
		return err
	}
	kf, err := auth.NewJWKSKeyfunc(ctx, cfg)
	if err != nil {
		return eris.Wrap(err, "run")
	}
	authMW, err := auth.AuthMiddleware(kf, cfg)
	if err != nil {
		return eris.Wrap(err, "run")
	}

	dbCfg, err := database.ConfigFromEnv()
	if err != nil {
		return eris.Wrap(err, "run: db config")
	}
	notionEnv, missingNotionEnv, err := notionsync.OptionalConfigFromEnv()
	if err != nil {
		return err
	}
	notionSyncDisabled := len(missingNotionEnv) > 0
	if notionSyncDisabled {
		slog.WarnContext(ctx, "notion sync: disabled, missing env vars",
			"missing_vars", missingNotionEnv,
		)
	}
	if err := database.Migrate(dbCfg.URL); err != nil {
		return eris.Wrap(err, "run: migrate")
	}
	db, err := database.Open(ctx, dbCfg)
	if err != nil {
		return eris.Wrap(err, "run: db open")
	}

	repos := newAppRepos(db, logger)
	authSvc := auth.NewService(repos.userRole)
	adminGate := usecase.NewAdminGate(authSvc)

	promoter, err := bootstrapSuperUserPromoter(ctx, logger, authSvc, repos.role, repos.userRole, os.Getenv("SUPER_USER_EMAILS"))
	if err != nil {
		return err
	}

	resolvers, pingHandler, notionSyncHandler, err := buildResolver(repos, authSvc, adminGate, logger, notionEnv, notionSyncDisabled, pingToken)
	if err != nil {
		return err
	}
	// newRouter must be called after telemetry.Init: the otelhttp handler it
	// constructs reads otel.GetTextMapPropagator() eagerly (it is newRouter, not
	// the buildResolver call just above, that constructs that handler). See comment
	// above telemetry.Init for the full ordering invariant.
	e := newRouter(resolvers, authMW, promoter, repos.loaderDeps(), pingHandler, notionSyncHandler, srvCfg.introspectionEnabled)
	e.Logger = logger

	port := os.Getenv("PORT")
	if port == "" {
		port = "1323"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           e,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		logger.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	g.Go(func() error {
		<-gctx.Done()
		timeout := srvCfg.shutdownTimeout
		logger.Info("shutdown signal received", "timeout", timeout.String())
		sctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		// Shutdown HTTP first so in-flight requests release DB conns before we
		// close the pool. db.Close is called unconditionally afterwards so the
		// pool is released even if Shutdown times out.
		shutdownErr := srv.Shutdown(sctx)
		db.Close()
		if tracerErr := tracerShutdown(sctx); tracerErr != nil {
			logger.Warn("tracer shutdown failed", "err", tracerErr)
		}
		if shutdownErr != nil {
			if errors.Is(shutdownErr, context.DeadlineExceeded) {
				logger.Warn("graceful shutdown timed out, forcing close", "timeout", timeout.String())
				return nil
			}
			return shutdownErr
		}
		return nil
	})

	return g.Wait()
}

func main() {
	logger := slog.New(logging.NewContextHandler(slog.NewJSONHandler(os.Stderr, nil), internalmw.RequestIDFromContext))
	slog.SetDefault(logger)

	// Load .env.local for local dev. Overload (not Load) so .env.local wins
	// over inherited shell env: mise auto-exports the root .env (production
	// template, with empty placeholders for keys like NOTION_MASTER_CARDGROUP_NAME),
	// which would otherwise mask the local-dev values written by
	// `make notion-local-setup`. Production (Render) is unaffected because
	// .env.local is gitignored and never deployed.
	_ = godotenv.Overload(".env.local")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logging.LogError(ctx, logger, "server terminated", err)
		os.Exit(1)
	}
}
