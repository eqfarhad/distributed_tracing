// Copyright Hashem Taheri

package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"medium-opentelemetry-poc/lib/tracing"
	"medium-opentelemetry-poc/queryyer/people"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	"go.opentelemetry.io/otel/propagation"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
	"go.opentelemetry.io/otel/trace"
)

var repo *people.Repository

const (
	service     = "queryyer"
	environment = "production"
	id          = 1
)

var tracer = otel.Tracer("queryyer-service")

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

	//Main functionality
	repo = people.NewRepository()
	defer repo.Close()

	wrappedHandler := otelhttp.NewHandler(http.HandlerFunc(handleGetPerson), "/getPerson/")
	http.Handle("/getPerson/", wrappedHandler)

	log.Print("Listening on :8081/")
	log.Fatal(http.ListenAndServe(":8081", nil))

}

func handleGetPerson(w http.ResponseWriter, r *http.Request) {
	// UsernameKey which sent as baggage
	uk := attribute.Key("username")
	// Getting the context from the request
	ctx := r.Context()
	// Starting a new trace in continous of received one
	ctx, span := tracer.Start(ctx, "handleGetPerson")
	defer span.End()
	// Getting the value of the baggage
	username := baggage.Value(ctx, uk)
	// Creating an event
	span.AddEvent("handling this...", trace.WithAttributes(uk.String(username.AsString())))

	// getting the name out of api url
	name := strings.TrimPrefix(r.URL.Path, "/getPerson/")
	person, err := repo.GetPerson(ctx, name)
	log.Print("person", person)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "handleGetPerson-queryyer")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Print(person)
	bytes, err := json.Marshal(person)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Print("bytes", bytes)
	w.Write(bytes)
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func initProvider() {
	ctx := context.Background()

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "127.0.0.1:4317" // setting default endpoint for exporter
	}

	// Create new OTLP Exporter
	driver := otlpgrpc.NewDriver(
		otlpgrpc.WithInsecure(),
		otlpgrpc.WithEndpoint(endpoint),
		otlpgrpc.WithDialOption(),
		//otlpgrpc.WithDialOption(grpc.WithBlock()), // useful for testing
	)
	exporter, err := otlp.NewExporter(ctx, driver)
	handleErr(err, "failed to create new OTLP exporter")

	idg := xray.NewIDGenerator()

	service := os.Getenv("GO_GORILLA_SERVICE_NAME")
	if service == "" {
		service = "go-gorilla"
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceNameKey.String("queryyer"),
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
	// if you want to set the meter (you need also enable it in the agent config)
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
