package middleware

import (
	"context"
	"net/http"
	"slices"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

type tracingOptions struct {
	provider   trace.TracerProvider
	propagator propagation.TextMapPropagator
	skipPaths  []string
}

type TracingOption func(*tracingOptions)

func WithTracerProvider(provider trace.TracerProvider) TracingOption {
	return func(opts *tracingOptions) { opts.provider = provider }
}

func WithPropagator(propagator propagation.TextMapPropagator) TracingOption {
	return func(opts *tracingOptions) { opts.propagator = propagator }
}

func WithSkipPaths(paths ...string) TracingOption {
	paths = slices.Clone(paths)
	return func(opts *tracingOptions) { opts.skipPaths = append(opts.skipPaths, paths...) }
}

// Tracing creates a server span and makes it available through the request context.
func Tracing(serviceName string, opts ...TracingOption) core.HandlerFunc {
	var config tracingOptions
	for _, option := range opts {
		option(&config)
	}
	if config.provider == nil {
		config.provider = otel.GetTracerProvider()
	}
	tracer := config.provider.Tracer(serviceName)
	return func(c *core.Context) {
		if slices.Contains(config.skipPaths, c.Request.URL.Path) {
			c.Next()
			return
		}
		parent := extractTraceContext(c.Request, config.propagator)
		method, route := c.Request.Method, c.FullPath()
		name := method
		attributes := []attribute.KeyValue{attribute.String("http.request.method", method)}
		if route != "" {
			name += " " + route
			attributes = append(attributes, attribute.String("http.route", route))
		}
		ctx, span := tracer.Start(
			parent,
			name,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(attributes...),
		)
		defer span.End()
		c.Request = c.Request.WithContext(ctx)
		c.Next()

		status := c.Writer.Status()
		span.SetAttributes(attribute.Int("http.response.status_code", status))
		if status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, "")
		}
	}
}

func extractTraceContext(request *http.Request, propagator propagation.TextMapPropagator) context.Context {
	ctx := request.Context()
	carrier := propagation.HeaderCarrier(request.Header)
	if propagator != nil {
		return propagator.Extract(ctx, carrier)
	}
	global := otel.GetTextMapPropagator()
	if len(global.Fields()) == 0 {
		return propagation.TraceContext{}.Extract(ctx, carrier)
	}
	extracted := global.Extract(ctx, carrier)
	if carrier.Get("traceparent") != "" &&
		trace.SpanContextFromContext(extracted).Equal(trace.SpanContextFromContext(ctx)) {
		return propagation.TraceContext{}.Extract(extracted, carrier)
	}
	return extracted
}
