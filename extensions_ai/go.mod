module github.com/futugyou/extensions_ai

go 1.26.2

require (
	github.com/futugyou/yomawari v0.0.1
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/openai/openai-go/v3 v3.42.0
	go.opentelemetry.io/otel v1.44.0
	go.opentelemetry.io/otel/metric v1.44.0
	go.opentelemetry.io/otel/trace v1.44.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
)

replace github.com/futugyou/yomawari v0.0.1 => ../yomawari
