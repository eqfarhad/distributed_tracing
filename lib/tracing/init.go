package tracing

import (
	"log"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
)

// tracerProvider returns an OpenTelemetry TracerProvider configured to use
// the Jaeger exporter that will send spans to the provided url. The returned
// TracerProvider will also use a Resource configured with all the information
// about the application.
func TracerProvider(collectorUrl string, agentHostName string, agentPort string, service string, environment string, id int64) (*tracesdk.TracerProvider, error) {
	log.Println("Agent Hostname=", agentHostName)
	log.Println("agentport=", agentPort)
	// Create the Jaeger exporter
	// Exporters are packages that allow telemetry data to be emitted somewhere
	// Sending to the agent
	// exp, err := jaeger.NewRawExporter(jaeger.WithAgentEndpoint(jaeger.WithAgentHost("jaeger-agent"), jaeger.WithAgentPort("5775")))
	// exp, err := jaeger.NewRawExporter(jaeger.WithAgentEndpoint())
	// if we don't specify the name gonna be localhost(work in local scenario and tests)
	//  exp, err := jaeger.NewRawExporter(jaeger.WithAgentEndpoint(jaeger.WithAgentPort("5775")))
	// in case of docker image we need to specify and pass the both (agent name and port)
	exp, err := jaeger.NewRawExporter(jaeger.WithAgentEndpoint(jaeger.WithAgentHost(agentHostName), jaeger.WithAgentPort(agentPort)))

	// Sending directly to collector (optional)
	// exp, err := jaeger.NewRawExporter(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(collectorUrl)))
	// Use http://localhost:14268/api/traces as default Jaeger collector endpoint instead of http://localhost:14250. (#1898)
	// This gonna be fixed in next release and we don't need to path the url to here
	//exp, err := jaeger.NewRawExporter(jaeger.WithCollectorEndpoint())

	if err != nil {
		return nil, err
	}

	// This block of code will create a new batch span processor,
	// a type of span processor that batches up multiple spans over a period of time, that writes to the exporter we created in the above
	bsp := tracesdk.NewBatchSpanProcessor(exp)
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithSpanProcessor(bsp),
		// Default is always sample
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
		// same as using bsp (shorter way)
		// tracesdk.WithBatcher(exp),
		// Record information about this application in an Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.ServiceNameKey.String(service),
			attribute.String("environment", environment),
			attribute.Int64("ID", id),
		),
		),
	)
	return tp, nil
}
