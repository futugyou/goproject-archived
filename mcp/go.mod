module github.com/futugyou/mcp

go 1.26.2

require (
	github.com/futugyou/extensions_ai v0.0.1
	github.com/futugyou/yomawari v0.0.1
	github.com/google/uuid v1.6.0
	go.opentelemetry.io/otel v1.43.0
	go.opentelemetry.io/otel/metric v1.43.0
	go.opentelemetry.io/otel/trace v1.43.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
)

replace github.com/futugyou/yomawari v0.0.1 => ../yomawari

replace github.com/futugyou/extensions_ai v0.0.1 => ../extensions_ai
