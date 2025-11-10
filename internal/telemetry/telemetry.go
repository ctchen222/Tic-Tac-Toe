package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// InitOtel initializes an OpenTelemetry SDK with configurations for traces, metrics, and logs.
func InitOtel() (func(context.Context) error, error) {
	ctx := context.Background()

	// Get OTLP endpoint from environment variable or use default
	otelEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otelEndpoint == "" {
		otelEndpoint = "localhost:4317"
	}

	// --- Create gRPC connection ---
	conn, err := grpc.DialContext(ctx, otelEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to OTLP collector: %w", err)
	}

	// --- Create shared resource ---
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("tic-tac-toe"),
			semconv.ServiceVersion("v0.1.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// --- Setup Traces ---
	otlpTraceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(otlpTraceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// --- Setup Metrics ---
	otlpMetricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(otlpMetricExporter)),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// --- Setup Logs ---
	otlpLogExporter, err := otlploggrpc.New(ctx, otlploggrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP log exporter: %w", err)
	}

	lp := log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(otlpLogExporter)),
		log.WithResource(res),
	)
	global.SetLoggerProvider(lp)

	// --- Set Propagators ---
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// --- Shutdown function ---
	shutdown := func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// Shutdown providers
		if err := tp.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown TracerProvider: %w", err)
		}
		if err := mp.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown MeterProvider: %w", err)
		}
		if err := lp.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown LoggerProvider: %w", err)
		}

		// Close the gRPC connection
		if err := conn.Close(); err != nil {
			return fmt.Errorf("failed to close gRPC connection: %w", err)
		}

		return nil
	}

	return shutdown, nil
}
