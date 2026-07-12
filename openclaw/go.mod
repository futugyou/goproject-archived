module github.com/futugyou/openclaw

go 1.26.2

require github.com/google/uuid v1.6.0

require (
	github.com/flosch/pongo2/v7 v7.0.0-alpha.1
	github.com/futugyou/extensions_ai v0.0.1
	github.com/hibiken/asynq v0.26.0
	github.com/jinzhu/copier v0.4.0
	github.com/mattn/go-sqlite3 v1.14.47
	github.com/robfig/cron/v3 v3.0.1
	go.opentelemetry.io/otel v1.44.0
	go.opentelemetry.io/otel/metric v1.44.0
	go.opentelemetry.io/otel/trace v1.44.0
	gorm.io/driver/postgres v1.6.0
	gorm.io/gorm v1.31.2
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.9.2 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/redis/go-redis/v9 v9.14.1 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

replace github.com/futugyou/extensions_ai v0.0.1 => ../extensions_ai
