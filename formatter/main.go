// Copyright Hashem Taheri

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"medium-opentelemetry-poc/lib/tracing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	"go.opentelemetry.io/otel/propagation"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
)

const (
	service     = "formatter-service"
	environment = "backend"
	id          = 2
)

var tracer = otel.Tracer("formatter-service")

func main() {
	// We have two configuration, either using otel collector as agent/collector
	// or using the jaeger agent/collector, to export traces to
	tracingOption := getenv("TRACING_OPTION", "otel-collector")

	if tracingOption == "otel-collector" {
		initProvider()
	} else if tracingOption == "jaeger-collector" {
		initProviderJaeger()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
	}(ctx)

	wrappedHandler := otelhttp.NewHandler(http.HandlerFunc(handleFormatGreeting), "/formatGreeting/")
	http.Handle("/formatGreeting/", wrappedHandler)

	log.Print("Listening on :8082/")
	log.Fatal(http.ListenAndServe(":8082", nil))
}

func handleFormatGreeting(w http.ResponseWriter, r *http.Request) {
	// Getting the context from the request
	ctx := r.Context()
	// Starting a new trace in continous of received one
	ctx, span := tracer.Start(ctx, "formatter-handleFormatGreeting")
	defer span.End()

	name := r.FormValue("name")
	title := r.FormValue("title")
	descr := r.FormValue("description")

	greeting := FormatGreeting(ctx, name, title, descr)
	w.Write([]byte(greeting))
}

// FormatGreeting combines information about a person into a greeting string.
func FormatGreeting(ctx context.Context, name, title, description string) string {
	ctx, span := tracer.Start(ctx, "formatter_formatGreeting_function")
	defer span.End()

	response := "Hello, "
	if title != "" {
		response += title + " "
	}
	response += name + "!"
	if description != "" {
		response += " " + description
	}
	return response
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func initProvider() {
	log.Print("initStarted")
	ctx := context.Background()

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		// setting default endpoint for exporter
		// in case of sidecar gonna work as well
		endpoint = "127.0.0.1:4317"
	}

	// Create new OTLP Exporter
	driver := otlpgrpc.NewDriver(
		otlpgrpc.WithInsecure(),
		otlpgrpc.WithEndpoint(endpoint),
		otlpgrpc.WithDialOption(), // useful for testing
	)
	exporter, err := otlp.NewExporter(ctx, driver)
	handleErr(err, "failed to create new OTLP exporter")

	idg := xray.NewIDGenerator()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceNameKey.String("formatter"),
		),
	)
	handleErr(err, "failed to create resource")

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSyncer(exporter),
		sdktrace.WithIDGenerator(idg),
	)

	cont := controller.New(
		processor.New(
			simple.NewWithExactDistribution(),
			exporter,
		),
		controller.WithExporter(exporter),
		controller.WithCollectPeriod(2*time.Second),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(xray.Propagator{})
	// if you want to set Meter
	// global.SetMeterProvider(cont.MeterProvider())
	_ = cont.Start(ctx)
}

func initProviderJaeger() {
	jaegerCollectorURL := getenv("JAEGER_COLLECTOR_URL", "http://localhost:14268/api/traces")
	jaegerAgenthost := getenv("JAEGER_AGENT_NAME", "localhost")
	jaegerAgentport := getenv("JAEGER_AGENT_PORT", "5775")
	tp, err := tracing.TracerProvider(jaegerCollectorURL, jaegerAgenthost, jaegerAgentport, service, environment, id)
	if err != nil {
		log.Fatal(err)
	}
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

}

func handleErr(err error, message string) {
	if err != nil {
		log.Fatalf("%s: %v", message, err)
	}
}
