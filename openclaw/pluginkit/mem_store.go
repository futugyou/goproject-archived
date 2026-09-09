package pluginkit

import (
	"github.com/futugyou/openclaw/core"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TODO
type PostgresMemoryStore struct {
	db           *gorm.DB
	config       core.MemoryMempalaceConfig
	sessionStore *core.PostgresMemoryStore
	embedder     *HashingEmbedder
}

func NewPostgresMemoryStore(config core.GatewayConfig, metrics *core.RuntimeMetrics) *PostgresMemoryStore {
	db, err := gorm.Open(postgres.Open(config.Memory.Postgres.PostgresUrl), &gorm.Config{})
	if err != nil {
		panic(err.Error())
	}

	sessionstore, err := core.NewPostgresMemoryStore(db, false, true, nil)
	if err != nil {
		panic(err.Error())
	}

	return &PostgresMemoryStore{
		db:           db,
		config:       *config.Memory.Mempalace,
		sessionStore: sessionstore,
		embedder:     &HashingEmbedder{Dimensions: config.Memory.Mempalace.EmbeddingDimensions},
	}
}
