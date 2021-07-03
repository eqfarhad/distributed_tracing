// Copyright Hashem Taheri

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"medium-opentelemetry-poc/lib/model"
	"medium-opentelemetry-poc/lib/tracing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

	"go.opentelemetry.io/contrib/propagators/aws/xray"
)

const (
	service     = "Server-helloWorld"
	environment = "production"
	id          = 1
)

var tracer = otel.Tracer("main-service")

func main() {

	// We have two configuration, either using otel collector as agent/collector
	// or using the jaeger agent/collector, to export traces
	tracingOption := getenv("TRACING_OPTION", "otel-collector")

	if tracingOption == "otel-collector" {
		initProvider()
	} else if tracingOption == "jaeger-collector" {
		initProviderJaeger()
	}

	// Important to defer the cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
	}(ctx)

	// calling Handle function, which is wrapped for tracing
	wrappedHandler := otelhttp.NewHandler(http.HandlerFunc(handleSayHello), "/sayHello/")
	http.Handle("/sayHello/", wrappedHandler)
	// unwrapped HandleFunc is like below
	// http.HandleFunc("/sayHello/", handleSayHello)
	listeningPort := getenv("PORT", ":8080")
	log.Print("Listening on http://localhost:8080/")
	log.Fatal(http.ListenAndServe(listeningPort, nil))

}

func handleSayHello(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Here we are adding more information to the auto instrumented trace span (optional)
	// span := trace.SpanFromContext(ctx)

	// To have sperated span (child span):
	// we can comment the code below to have the current extra information as part of
	// the span which already began by plugin
	ctx, span := tracer.Start(ctx, "handleSayHello")
	log.Printf("mainServer TraceID=%t", span.SpanContext().HasTraceID())
	log.Printf("main Server TraceID=%s", span.SpanContext().TraceID())
	// // Don't forget to end span!
	defer span.End()
	// Adding attributes (tags)
	span.SetAttributes(attribute.Key("MoreInfo").String("ca va?"))
	//simulating an error
	span.RecordError(errors.New("Opps"))
	// For very sensetive error, we can change status to error
	// So in the ui we will have visually informed
	span.SetStatus(codes.Error, "Oh No!")

	// we can also add event (added to logging part)
	span.AddEvent("example Event", trace.WithAttributes(
		attribute.String("first Item", "First Value"),
	))

	name := strings.TrimPrefix(r.URL.Path, "/sayHello/")
	greeting, err := SayHello(ctx, name)
	if err != nil {
		span.SetAttributes(attribute.Bool("error", true))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// span.SetTag("response", greeting)
	w.Write([]byte(greeting))
}

// SayHello creates a greeting for the named person.
func SayHello(ctx context.Context, name string) (string, error) {
	ctx, span := tracer.Start(ctx, "main_SayHello_function")
	span.SetAttributes(attribute.String("name", name))
	defer span.End()

	person, err := getPerson(ctx, name)
	if err != nil {
		return "", err
	}

	return formatGreeting(ctx, person)
}

func getPerson(ctx context.Context, name string) (*model.Person, error) {
	queryyerURL := getenv("QUERYYER_URL", "http://localhost:8081/getPerson/")
	// queryyerURL_java := getenv("QUERYYER_URL", "http://localhost:8081/getPerson?name=")
	log.Print("querryerURL=\n", queryyerURL)

	ctx, span := tracer.Start(ctx, "main_getPerson_function")
	span.SetAttributes(attribute.String("name", name))
	defer span.End()

	url := queryyerURL + name
	res, err := get(ctx, "getPerson", url)
	var person model.Person
	if err = json.Unmarshal(res, &person); err != nil {
		return nil, err
	}
	return &person, nil
}

func formatGreeting(ctx context.Context, person *model.Person) (string, error) {
	formatterURL := getenv("FORMATTER_URL", "http://localhost:8082/formatGreeting?")

	ctx, span := tracer.Start(ctx, "main_formatGreeting_function")
	span.SetAttributes(attribute.String("person.Name", person.Name))
	defer span.End()

	v := url.Values{}
	v.Set("name", person.Name)
	v.Set("title", person.Title)
	v.Set("description", person.Description)

	span.AddEvent("formatGreeting-recived-values", trace.WithAttributes(attribute.Array(
		"url-values", []string{person.Name, person.Description, person.Title},
	)))

	url := formatterURL + v.Encode()
	res, err := get(ctx, "formatGreeting", url)
	// log.Print(res)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func get(ctx context.Context, operationName, url string) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "main-get-function")
	// Don't forget to end span!
	defer span.End()

	// NewTransport wraps the provided http.RoundTripper with one that starts a span
	// and injects the span context into the outbound request headers.
	var httpClient = http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	// ContextWithValues returns a copy of parent with pairs updated in the baggage.
	ctx = baggage.ContextWithValues(ctx,
		attribute.String("username", "donuts"),
	)
	// using additional httptrace plugin for tracing http (Super detail traces then about HTTP connection ;D )
	// ctx = httptrace.WithClientTrace(ctx, otelhttptrace.NewClientTrace(ctx))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	// Do request
	log.Printf("Sending request...%s\n", operationName)
	return DoWithClient(req, &httpClient)
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

// DoWithClient executes an HTTP request and returns the response body.
// Any errors or non-200 status code result in an error.
func DoWithClient(req *http.Request, client *http.Client) ([]byte, error) {
	_, span := tracer.Start(req.Context(), "DoWithClient")
	// // Don't forget to end span!
	defer span.End()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Printf("Response Received: %s\n", body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("StatusCode: %d, Body: %s", resp.StatusCode, body)
	}

	return body, nil
}

func initProvider() {
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
		otlpgrpc.WithDialOption(),
		// otlpgrpc.WithDialOption(grpc.WithBlock()), // useful for testing/debuging
		// because it's not going to pass this line if it couldn't find and connect to the agent
	)
	exporter, err := otlp.NewExporter(ctx, driver)
	handleErr(err, "failed to create new OTLP exporter")

	// if you want to have specific kind of trace ID,
	// for instance if you want to set up the otel collector to export traces to both aws cloudwatch
	// and another jaeger instance
	idg := xray.NewIDGenerator()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceNameKey.String("main"),
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
	// if you are using the meter (you need also enable it in the agent config)
	// global.SetMeterProvider(cont.MeterProvider())
	_ = cont.Start(ctx)
}

func initProviderJaeger() {

	// We get the jaeger collector endpoint (in case we want to send traces straightly to the collector)
	jaegerCollectorURL := getenv("JAEGER_COLLECTOR_URL", "http://localhost:14268/api/traces")
	// Getting the Agent information
	jaegerAgenthost := getenv("JAEGER_AGENT_NAME", "localhost")
	jaegerAgentport := getenv("JAEGER_AGENT_PORT", "5775")

	// Created another package file for this part (as required some more comments)
	// tracing.TracerProvider returns an OpenTelemetry TracerProvider configured to use
	// the Jaeger exporter that will send spans to the provided url. The returned
	// TracerProvider will also use a Resource configured with all the information
	// about the application.
	tp, err := tracing.TracerProvider(jaegerCollectorURL, jaegerAgenthost, jaegerAgentport, service, environment, id)
	if err != nil {
		log.Fatal(err)
	}

	// Register our TracerProvider as the global so any imported
	// instrumentation in the future will default to using it.
	// SetTracerProvider registers `tp` as the global trace provider.
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

}

func handleErr(err error, message string) {
	if err != nil {
		log.Fatalf("%s: %v", message, err)
	}
}
