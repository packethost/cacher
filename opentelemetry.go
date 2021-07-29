package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"

	"github.com/packethost/pkg/env"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	"go.opentelemetry.io/otel/exporters/stdout"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
	"google.golang.org/grpc/credentials"
)

// initOtel sets up the OpenTelemetry plumbing so it's ready to use.
// Returns a func() that encapuslates clean shutdown.
// Configured via environment:
// OTEL_EXPORTER_OTLP_ENDPOINT an OTLP endpoint URI or "stdout" to print tracing data to stdout
// OTEL_EXPORTER_OTLP_INSECURE set to true to enable unencrypted OTLP (e.g. localhost collector)
func initOtel() func() {
	otlpEndpoint := env.Get("OTEL_EXPORTER_OTLP_ENDPOINT")
	otlpInsecure := env.Bool("OTEL_EXPORTER_OTLP_INSECURE")
	ctx := context.Background()

	// set the service name that will show up in tracing UIs
	resAttrs := resource.WithAttributes(semconv.ServiceNameKey.String("cacher"))
	res, err := resource.New(ctx, resAttrs)
	if err != nil {
		log.Fatalf("failed to create OpenTelemetry service name resource: %s", err)
	}

	// might be OTLP, might be stdout (to dev null, to prevent errors when unconfigured)
	var exporter sdktrace.SpanExporter

	if otlpEndpoint != "" {
		driverOpts := []otlpgrpc.Option{otlpgrpc.WithEndpoint(otlpEndpoint)}
		if otlpInsecure {
			driverOpts = append(driverOpts, otlpgrpc.WithInsecure())
		} else {
			creds := credentials.NewClientTLSFromCert(nil, "")
			driverOpts = append(driverOpts, otlpgrpc.WithTLSCredentials(creds))
		}

		driver := otlpgrpc.NewDriver(driverOpts...)
		exporter, err = otlp.NewExporter(ctx, driver)
		if err != nil {
			log.Fatalf("failed to configure OTLP exporter: %s", err)
		}
	} else if otlpEndpoint == "stdout" {
		// `--otlp-endpoint stdout` will print traces to stdout
		exporter, err = stdout.NewExporter(stdout.WithWriter(os.Stdout))
		if err != nil {
			log.Fatalf("failed to configure stdout exporter: %s", err)
		}
	} else {
		// this sets up the stdout exporter so all the plumbing comes up as usual
		// but the data is discarded immediately, so that when there is no OTLP
		// endpoint configured, there are no errors or interruption of service
		exporter, err = stdout.NewExporter(stdout.WithWriter(ioutil.Discard))
		if err != nil {
			log.Fatalf("failed to configure stdout as null exporter: %s", err)
		}
	}

	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	// set global propagator to tracecontext (the default is no-op).
	prop := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	otel.SetTextMapPropagator(prop)

	// inject the tracer into the otel globals, start background goroutines
	otel.SetTracerProvider(tracerProvider)

	// callers need to defer this to make sure all the data gets flushed out
	return func() {
		err = tracerProvider.Shutdown(ctx)
		if err != nil {
			log.Fatalf("shutdown of OpenTelemetry tracerProvider failed: %s", err)
		}

		err = exporter.Shutdown(ctx)
		if err != nil {
			log.Fatalf("shutdown of OpenTelemetry OTLP exporter failed: %s", err)
		}
	}
}
