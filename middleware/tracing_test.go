package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestTracingCreatesServerSpansWithHTTPMetadata(t *testing.T) {
	previous := otel.GetTextMapPropagator()
	t.Cleanup(func() { otel.SetTextMapPropagator(previous) })
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())

	for _, test := range []struct {
		name             string
		target           string
		header           string
		propagator       propagation.TextMapPropagator
		globalPropagator propagation.TextMapPropagator
		route            string
		status           int
		skip             bool
	}{
		{name: "fallback", target: "/items/1", header: "traceparent", route: "/items/:id", status: http.StatusNoContent},
		{name: "five xx", target: "/items/fail", header: "traceparent", route: "/items/:id", status: http.StatusBadGateway},
		{name: "unmatched", target: "/random-scan", header: "traceparent", status: http.StatusNotFound},
		{
			name: "global without trace extraction", target: "/items/1", header: "traceparent",
			globalPropagator: propagation.Baggage{}, route: "/items/:id", status: http.StatusNoContent,
		},
		{
			name: "explicit propagator", target: "/items/1", header: "x-test-trace",
			propagator: customTraceHeader{}, route: "/items/:id", status: http.StatusNoContent,
		},
		{name: "skip", target: "/healthz?probe=1", header: "traceparent", status: http.StatusOK, skip: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.globalPropagator != nil {
				old := otel.GetTextMapPropagator()
				t.Cleanup(func() { otel.SetTextMapPropagator(old) })
				otel.SetTextMapPropagator(test.globalPropagator)
			}
			tracer := &recordingServerTracer{}
			provider := &recordingTracerProvider{tracer: tracer}
			engine := gin.New()
			engine.Use(
				Tracing(
					"items-service",
					WithTracerProvider(provider),
					WithPropagator(test.propagator),
					WithSkipPaths("/healthz"),
				),
			)
			observeSpan := func(c *core.Context) {
				span := trace.SpanContextFromContext(c.Request.Context())
				if test.skip {
					if span.IsValid() {
						t.Errorf("skipped request received a span: %v", span)
					}
				} else if !span.Equal(tracer.span.SpanContext()) || !span.IsValid() || span.IsRemote() {
					t.Errorf("handler did not receive the local span: %v", span)
				}
				c.Status(test.status)
			}
			engine.GET("/items/:id", observeSpan)
			engine.GET("/healthz", observeSpan)
			engine.NoRoute(observeSpan)

			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			request.Header.Set(test.header, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
			request.Header.Set("tracestate", "vendor=value")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("HTTP status = %d, want %d", response.Code, test.status)
			}
			if test.skip {
				if tracer.span != nil {
					t.Fatal("skipped request started a span")
				}
				return
			}
			if tracer.parent.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" ||
				tracer.parent.SpanID().String() != "00f067aa0ba902b7" ||
				tracer.parent.TraceState().String() != "vendor=value" || !tracer.parent.IsRemote() {
				t.Fatalf("unexpected extracted parent: %v", tracer.parent)
			}
			wantName := http.MethodGet
			if test.route != "" {
				wantName += " " + test.route
			}
			wantStatus := codes.Unset
			if test.status >= http.StatusInternalServerError {
				wantStatus = codes.Error
			}
			span := tracer.span
			if provider.service != "items-service" || span.name != wantName ||
				span.kind != trace.SpanKindServer {
				t.Fatalf("unexpected server span: %+v", span)
			}
			if span.status != wantStatus || !span.ended {
				t.Fatalf("unexpected completed span: %+v", span)
			}
			attributes := span.attributes
			if attributes["http.route"].AsString() != test.route ||
				attributes["http.request.method"].AsString() != http.MethodGet ||
				attributes["http.response.status_code"].AsInt64() != int64(test.status) {
				t.Fatalf("unexpected HTTP span attributes: %v", attributes)
			}
		})
	}
}

type customTraceHeader struct{ propagation.TraceContext }

func (customTraceHeader) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier{
		"traceparent": carrier.Get("x-test-trace"),
		"tracestate":  carrier.Get("tracestate"),
	})
}

type recordingTracerProvider struct {
	noop.TracerProvider
	tracer  *recordingServerTracer
	service string
}

func (provider *recordingTracerProvider) Tracer(service string, _ ...trace.TracerOption) trace.Tracer {
	provider.service = service
	return provider.tracer
}

type recordingServerTracer struct {
	noop.Tracer
	parent trace.SpanContext
	span   *recordedServerSpan
}

func (tracer *recordingServerTracer) Start(
	ctx context.Context,
	name string,
	opts ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	tracer.parent = trace.SpanContextFromContext(ctx)
	config := trace.NewSpanStartConfig(opts...)
	span := &recordedServerSpan{
		name:       name,
		kind:       config.SpanKind(),
		attributes: make(map[attribute.Key]attribute.Value),
		context: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: tracer.parent.TraceID(),
			SpanID:  trace.SpanID{1},
		}),
	}
	span.SetAttributes(config.Attributes()...)
	tracer.span = span
	return trace.ContextWithSpan(ctx, span), span
}

type recordedServerSpan struct {
	noop.Span
	name       string
	kind       trace.SpanKind
	context    trace.SpanContext
	status     codes.Code
	attributes map[attribute.Key]attribute.Value
	ended      bool
}

func (span *recordedServerSpan) SpanContext() trace.SpanContext { return span.context }

func (span *recordedServerSpan) End(...trace.SpanEndOption) { span.ended = true }

func (span *recordedServerSpan) SetStatus(code codes.Code, _ string) {
	span.status = code
}

func (span *recordedServerSpan) SetAttributes(attrs ...attribute.KeyValue) {
	for _, attr := range attrs {
		span.attributes[attr.Key] = attr.Value
	}
}
