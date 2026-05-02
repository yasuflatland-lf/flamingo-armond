package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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

	"backend/graph/generated"
	"backend/graph/resolver"
	"backend/internal/auth"
	"backend/internal/database"
	"backend/internal/domain/service"
	"backend/internal/handler/ping"
	"backend/internal/loader"
	"backend/internal/logging"
	internalmw "backend/internal/middleware"
	"backend/internal/repository"
	"backend/internal/telemetry"
	"backend/internal/usecase"
)

const defaultShutdownTimeout = 25 * time.Second

func newGraphQLServer(r *resolver.Resolver) *handler.Server {
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.POST{})

	srv.Use(extension.FixedComplexityLimit(100))

	if os.Getenv("GRAPHQL_INTROSPECTION") != "off" {
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
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	cardgroupRepo repository.CardgroupRepository,
	cardRepo repository.CardRepository,
	pingHandler *ping.Handler,
	swipeRecordRepo ...repository.SwipeRecordRepository,
) *echo.Echo {
	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(internalmw.RequestID())

	e.GET("/", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"service": "flamingo-armond-backend",
		})
	})

	e.GET("/health", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	e.POST("/internal/ping", pingHandler.Handle, pingHandler.RateLimiter())

	gqlSrv := newGraphQLServer(resolvers)
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
	q := e.Group("/query", authMW, promoter.Middleware(), loader.Middleware(userRepo, roleRepo, cardgroupRepo, cardRepo, swipeRecordRepo...))
	q.POST("", echo.WrapHandler(otelGQLHandler))
	e.GET("/playground", echo.WrapHandler(playground.Handler("GraphQL", "/query")))

	return e
}

func swipeNextBatchSize(logger *slog.Logger) int {
	v := os.Getenv("SWIPE_NEXT_BATCH_SIZE")
	if v == "" {
		return 10
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		logger.Warn("invalid SWIPE_NEXT_BATCH_SIZE, using default", "value", v, "default", 10)
		return 10
	}
	return n
}

func shutdownTimeout(logger *slog.Logger) time.Duration {
	v := os.Getenv("SHUTDOWN_TIMEOUT")
	if v == "" {
		return defaultShutdownTimeout
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		logger.Warn("invalid SHUTDOWN_TIMEOUT, using default",
			"value", v, "err", err, "default", defaultShutdownTimeout.String())
		return defaultShutdownTimeout
	}
	if d <= 0 {
		logger.Warn("non-positive SHUTDOWN_TIMEOUT, using default",
			"value", v, "default", defaultShutdownTimeout.String())
		return defaultShutdownTimeout
	}
	return d
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
	if err := database.Migrate(dbCfg.URL); err != nil {
		return eris.Wrap(err, "run: migrate")
	}
	db, err := database.Open(ctx, dbCfg)
	if err != nil {
		return eris.Wrap(err, "run: db open")
	}

	userRepo := repository.NewUserRepository(db.GORM)
	roleRepo := repository.NewRoleRepository(db.GORM)
	cardgroupRepo := repository.NewCardgroupRepository(db.GORM)
	cardRepo := repository.NewCardRepository(db.GORM)
	swipeRecordRepo := repository.NewSwipeRecordRepository(db.GORM)
	pingRecordRepo := repository.NewPingRecordRepository(db.GORM)
	userRoleRepo := repository.NewUserRoleRepository(db.GORM)
	authSvc := auth.NewService(userRoleRepo)

	// super-user auto-promote bootstrap
	superUserEmails := auth.ParseSuperUserSet(os.Getenv("SUPER_USER_EMAILS"))
	var promoter *auth.SuperUserPromoter
	if len(superUserEmails) > 0 {
		adminRole, err := roleRepo.FindByName(ctx, "admin")
		if err != nil {
			return eris.Wrap(err, "run: lookup admin role for super-user bootstrap")
		}
		promoter = auth.NewSuperUserPromoter(superUserEmails, adminRole.ID, authSvc, roleRepo)
		logger.Info("super-user bootstrap enabled", "email_count", len(superUserEmails))
	} else {
		promoter = auth.NewSuperUserPromoter(nil, "", nil, nil)
	}

	userUC := usecase.NewUserUsecase(userRepo)
	cardgroupUC := usecase.NewCardgroupUsecase(cardgroupRepo)
	cardUC := usecase.NewCardUsecase(db.GORM, cardRepo, cardgroupRepo)
	swipeUC := usecase.NewSwipeUsecase(db.GORM, cardRepo, cardgroupRepo, swipeRecordRepo, service.NewFSRSScheduler(), swipeNextBatchSize(logger))
	dictionaryUC := usecase.NewDictionaryUsecase(authSvc, cardRepo, db.GORM)
	adminUserUC := usecase.NewAdminUser(userRepo, roleRepo, authSvc)
	adminRoleUC := usecase.NewAdminRole(roleRepo, authSvc)
	lastViewedCardgroupUC := usecase.NewLastViewedCardgroup(userRepo)

	resolvers := resolver.NewResolver(userUC, cardgroupUC, cardUC, swipeUC, authSvc, dictionaryUC, adminUserUC, adminRoleUC, lastViewedCardgroupUC)
	pingHandler := ping.New(pingRecordRepo, pingToken)
	// newRouter must be called after telemetry.Init: the otelhttp handler it
	// constructs reads otel.GetTextMapPropagator() eagerly. See comment above
	// telemetry.Init for the full ordering invariant.
	e := newRouter(resolvers, authMW, promoter, userRepo, roleRepo, cardgroupRepo, cardRepo, pingHandler, swipeRecordRepo)
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
		timeout := shutdownTimeout(logger)
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

	// Load .env.local for local dev. godotenv.Load does not override
	// process-supplied env, so production (Render) keeps its platform-injected
	// values. Missing file is expected outside local dev and is not an error.
	_ = godotenv.Load(".env.local")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logging.LogError(ctx, logger, "server terminated", err)
		os.Exit(1)
	}
}
