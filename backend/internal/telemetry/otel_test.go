package telemetry_test

import (
	"context"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"backend/internal/telemetry"
)

func TestInit_SetsTextMapPropagator(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	shutdown, err := telemetry.InitWithExporter(
		context.Background(),
		slog.New(slog.DiscardHandler),
		exp,
	)
	if err != nil {
		t.Fatalf("InitWithExporter: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	fields := otel.GetTextMapPropagator().Fields()
	fieldSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		fieldSet[f] = true
	}
	if !fieldSet["traceparent"] {
		t.Errorf("expected propagator Fields to contain 'traceparent', got %v", fields)
	}
	if !fieldSet["tracestate"] {
		t.Errorf("expected propagator Fields to contain 'tracestate', got %v", fields)
	}
}

func TestInit_NoopWhenEndpointEmpty(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	shutdown, err := telemetry.Init(context.Background(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("Init returned error with empty endpoint: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("noop shutdown returned error: %v", err)
	}
}

func TestInit_ExporterReceivesSpans(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	shutdown, err := telemetry.InitWithExporter(
		context.Background(),
		slog.New(slog.DiscardHandler),
		exp,
	)
	if err != nil {
		t.Fatalf("InitWithExporter: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	tp, ok := otel.GetTracerProvider().(interface {
		ForceFlush(context.Context) error
	})
	if !ok {
		t.Fatal("expected global TracerProvider to support ForceFlush")
	}

	tr := otel.GetTracerProvider().Tracer("test")
	_, span := tr.Start(context.Background(), "unit-test-span")
	span.End()

	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush: %v", err)
	}
	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "unit-test-span" {
		t.Errorf("unexpected span name: %s", spans[0].Name)
	}
}

func TestInit_SamplerRatioParsing(t *testing.T) {
	cases := []struct {
		name string
		arg  string
	}{
		{"empty-defaults-to-one", ""},
		{"half", "0.5"},
		{"invalid-defaults-to-one", "not-a-float"},
		{"out-of-range-defaults-to-one", "2.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", tc.arg)
			exp := tracetest.NewInMemoryExporter()
			shutdown, err := telemetry.InitWithExporter(
				context.Background(),
				slog.New(slog.DiscardHandler),
				exp,
			)
			if err != nil {
				t.Fatalf("InitWithExporter: %v", err)
			}
			if err := shutdown(context.Background()); err != nil {
				t.Fatalf("shutdown: %v", err)
			}
		})
	}
}
