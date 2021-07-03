// Copyright Hashem Taheri

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"medium-opentelemetry-poc/lib/tracing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// This is just for automating the curl command and I'm exporting traces straightly to the jaeger collector

const (
	service     = "main-client"
	environment = "front-end"
	id          = 1
)

func main() {
	jaegerCollectorURL := getenv("JAEGER_COLLECTOR_URL", "http://localhost:14268/api/traces")
	serverURL := getenv("SERVER_URL", "http://localhost:8080/sayHello/hashem")
	jaegerAgenthost := getenv("JAEGER_AGENT_NAME", "localhost")
	jaegerAgentport := getenv("JAEGER_AGENT_PORT", "5775")

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
	// SetTextMapPropagator sets propagator as the global TextMapPropagator.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}(ctx)

	// Initialize one single request
	requestInit(ctx, &serverURL)
}

func requestInit(ctx context.Context, url *string) {
	// Here we are adding more information to the auto instrumented trace span
	// span := trace.SpanFromContext(ctx)
	var httpClient = http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	// To have sperated span (child span):
	// we can comment the code below to have the current extra information as part of
	// the span which already began by plugin
	ctx, span := otel.Tracer("Client").Start(ctx, "requestInit")
	log.Printf("TraceID=%t", span.SpanContext().HasTraceID())
	log.Printf("TraceID=%s", span.SpanContext().TraceID())
	// // Don't forget to end span!
	defer span.End()
	//ctx = httptrace.WithClientTrace(ctx, otelhttptrace.NewClientTrace(ctx))
	req, _ := http.NewRequestWithContext(ctx, "GET", *url, nil)

	fmt.Printf("Sending request...\n")
	res, _ := DoWithClient(req, &httpClient)
	fmt.Printf("Response Received:\n %s", res)
}

// DoWithClient executes an HTTP request and returns the response body.
// Any errors or non-200 status code result in an error.
func DoWithClient(req *http.Request, client *http.Client) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("StatusCode: %d, Body: %s", resp.StatusCode, body)
	}

	return body, nil
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}
