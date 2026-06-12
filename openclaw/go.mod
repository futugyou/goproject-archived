module github.com/futugyou/openclaw

go 1.26.2

require github.com/google/uuid v1.6.0

require (
	github.com/futugyou/extensions_ai v0.0.1
	go.opentelemetry.io/otel/trace v1.44.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
)

replace github.com/futugyou/extensions_ai v0.0.1 => ../extensions_ai
