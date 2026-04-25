// Package telemetry wires OpenTelemetry tracing for the backend.
//
// Init returns a shutdown function the caller must defer.
// When OTEL_EXPORTER_OTLP_ENDPOINT is empty, Init installs a no-op
// TracerProvider so dev / CI keep running without a collector.
package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"

	"github.com/rotisserie/eris"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type ShutdownFunc func(context.Context) error

const serviceName = "flamingo-armond-backend"

func noop() ShutdownFunc { return func(context.Context) error { return nil } }

func Init(ctx context.Context, logger *slog.Logger) (ShutdownFunc, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		logger.Info("telemetry disabled: OTEL_EXPORTER_OTLP_ENDPOINT is empty")
		return noop(), nil
	}
	exp, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
	if err != nil {
		return nil, eris.Wrap(err, "telemetry: new exporter")
	}
	shutdown, err := InitWithExporter(ctx, logger, exp)
	if err != nil {
		if shutdownErr := exp.Shutdown(ctx); shutdownErr != nil {
			logger.Warn("telemetry: exporter cleanup after init failure", "err", shutdownErr)
		}
		return nil, err
	}
	return shutdown, nil
}

func InitWithExporter(ctx context.Context, logger *slog.Logger, exp sdktrace.SpanExporter) (ShutdownFunc, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion()),
			semconv.DeploymentEnvironment(deploymentEnv()),
		),
	)
	if err != nil {
		if !errors.Is(err, resource.ErrPartialResource) && !errors.Is(err, resource.ErrSchemaURLConflict) {
			return nil, eris.Wrap(err, "telemetry: resource")
		}
		logger.Warn("telemetry: partial resource detection", "err", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler(logger)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	logger.Info("telemetry enabled", "sampler_arg", os.Getenv("OTEL_TRACES_SAMPLER_ARG"))
	return func(ctx context.Context) error { return tp.Shutdown(ctx) }, nil
}

func deploymentEnv() string {
	if v := os.Getenv("APP_ENV"); v != "" {
		return v
	}
	return "development"
}

func serviceVersion() string { return "dev" }

func sampler(logger *slog.Logger) sdktrace.Sampler {
	ratio := 1.0
	v := os.Getenv("OTEL_TRACES_SAMPLER_ARG")
	if v != "" {
		switch parsed, err := strconv.ParseFloat(v, 64); {
		case err != nil:
			logger.Warn("OTEL_TRACES_SAMPLER_ARG is not a float; defaulting to 1.0", "value", v)
		case parsed < 0 || parsed > 1:
			logger.Warn("OTEL_TRACES_SAMPLER_ARG is out of [0,1]; defaulting to 1.0", "value", v)
		default:
			ratio = parsed
		}
	}
	return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
}
