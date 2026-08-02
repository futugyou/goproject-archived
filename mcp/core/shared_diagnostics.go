package core

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "Experimental.ModelContextProtocol"
	metricName = "Experimental.ModelContextProtocol"
)

var (
	Tracer                       = otel.Tracer(tracerName)
	meter                        = otel.Meter(metricName)
	ShortSecondsBucketBoundaries = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
	LongSecondsBucketBoundaries  = []float64{0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30, 60, 120, 300}
)

var Propagator = otel.GetTextMapPropagator()

func CreateDurationHistogram(name string, description string, longBuckets bool) metric.Float64Histogram {
	boundaries := ShortSecondsBucketBoundaries
	if longBuckets {
		boundaries = LongSecondsBucketBoundaries
	}
	m, _ := meter.Float64Histogram(
		name,
		metric.WithUnit("s"),
		metric.WithDescription(description),
		metric.WithExplicitBucketBoundaries(boundaries...),
	)
	return m
}

func StartServerSpan(ctx context.Context, name string, carrier propagation.TextMapCarrier) (context.Context, trace.Span) {
	parentCtx := Propagator.Extract(ctx, carrier)
	link := trace.Link{SpanContext: trace.SpanContextFromContext(parentCtx)}
	return Tracer.Start(parentCtx, name,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithLinks(link),
	)
}

func StartSpanWithJsonRpcData(ctx context.Context, name string, message *JsonRpcMessage) (context.Context, trace.Span) {
	if message == nil {
		return Tracer.Start(ctx, name)
	}

	carrier := &JSONRPCHeaderCarrier{Message: message}
	parentCtx := Propagator.Extract(ctx, carrier)
	link := trace.Link{SpanContext: trace.SpanContextFromContext(parentCtx)}
	return Tracer.Start(parentCtx, name,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithLinks(link),
	)
}

func PropagatorInject(ctx context.Context, message *JsonRpcMessage) {
	if message == nil {
		return
	}

	carrier := &JSONRPCHeaderCarrier{Message: message}

	Propagator.Inject(ctx, carrier)
}

type JSONRPCHeaderCarrier struct {
	Message *JsonRpcMessage
}

func (c *JSONRPCHeaderCarrier) Set(key string, value string) {
	if c == nil || c.Message == nil || c.Message.IJsonRpcMessage == nil {
		return
	}

	switch req := c.Message.IJsonRpcMessage.(type) {
	case *JsonRpcRequest:
		newParams, err := injectMetaToParams(req.Params, key, value)
		if err == nil {
			req.Params = newParams
		}

	case *JsonRpcNotification:
		newParams, err := injectMetaToParams(req.Params, key, value)
		if err == nil {
			req.Params = newParams
		}
	}
}

func (c *JSONRPCHeaderCarrier) Get(key string) string {
	if c == nil || c.Message == nil || c.Message.IJsonRpcMessage == nil {
		return ""
	}

	var params json.RawMessage
	switch req := c.Message.IJsonRpcMessage.(type) {
	case *JsonRpcRequest:
		params = req.Params
	case *JsonRpcNotification:
		params = req.Params
	default:
		return ""
	}

	meta := extractMetaFromParams(params)
	if meta != nil {
		return meta[key]
	}
	return ""
}

func (c *JSONRPCHeaderCarrier) Keys() []string {
	if c == nil || c.Message == nil || c.Message.IJsonRpcMessage == nil {
		return nil
	}

	var params json.RawMessage
	switch req := c.Message.IJsonRpcMessage.(type) {
	case *JsonRpcRequest:
		params = req.Params
	case *JsonRpcNotification:
		params = req.Params
	default:
		return nil
	}

	meta := extractMetaFromParams(params)
	if meta == nil {
		return nil
	}

	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	return keys
}

func extractMetaFromParams(params json.RawMessage) map[string]string {
	if len(params) == 0 {
		return nil
	}

	// 1. 将 RawMessage 反序列化为 map[string]any
	var paramsMap map[string]any
	if err := json.Unmarshal(params, &paramsMap); err != nil {
		return nil
	}

	// 2. 尝试获取 _meta 字段
	metaVal, exists := paramsMap["_meta"]
	if !exists {
		return nil
	}

	// 3. 将 _meta 转换为 map[string]string
	metaBytes, err := json.Marshal(metaVal)
	if err != nil {
		return nil
	}

	var meta map[string]string
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil
	}

	return meta
}

func injectMetaToParams(params json.RawMessage, key, value string) (json.RawMessage, error) {
	var paramsMap map[string]any

	if len(params) == 0 {
		paramsMap = make(map[string]any)
	} else {
		if err := json.Unmarshal(params, &paramsMap); err != nil {
			return params, err
		}
	}

	var meta map[string]any
	if existingMeta, ok := paramsMap["_meta"].(map[string]any); ok {
		meta = existingMeta
	} else {
		meta = make(map[string]any)
		paramsMap["_meta"] = meta
	}

	meta[key] = value

	updatedParams, err := json.Marshal(paramsMap)
	if err != nil {
		return params, err
	}

	return updatedParams, nil
}
