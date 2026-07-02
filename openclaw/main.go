package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/hibiken/asynq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// test migrate
	dsn := os.Getenv("PostresDB_URL")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	db.AutoMigrate(
		&core.AutomationDefinition{},
		&core.Session{},
		&core.SessionBranch{},
		&core.Note{},
		&core.SessionSummary{},
		&core.SessionTurnsFts{},
	)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 初始化信号上下文
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 初始化核心调度器
	clawScheduler := core.NewClawLoopScheduler(logger)

	// asynq 消费者
	var concreteDispatcher core.IAgentLoopDispatcher = &core.NoopAgentLoopDispatcher{}
	agentHandler := core.NewAgentTaskHandler(concreteDispatcher, logger)

	mux := asynq.NewServeMux()
	mux.Handle(core.TypeAgentLoopTask, agentHandler)

	redisOpt := asynq.RedisClientOpt{Addr: os.Getenv("Redis_URL")}

	// 启动生产者
	go startScheduler(ctx, logger, redisOpt, clawScheduler)

	// asynq Server
	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 10,
	})

	if err := srv.Start(mux); err != nil {
		logger.Error("Failed to start asynq server", "err", err)
		return
	}
	logger.Info("Asynq worker server started successfully")

	<-ctx.Done()
	logger.Info("Shutting down gracefully...")

	srv.Shutdown()
	logger.Info("Server exited")
}

func startScheduler(ctx context.Context, logger *slog.Logger, redisOpt asynq.RedisClientOpt, clawScheduler *core.ClawLoopScheduler) {
	asynqClient := asynq.NewClient(redisOpt)
	defer func() {
		asynqClient.Close()
		logger.Info("Asynq client closed inside scheduler")
	}()

	job := core.NewAgentLoopJob(clawScheduler, asynqClient, logger)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Scheduler loop stopped")
			return

		case <-ticker.C:
			job.Execute(ctx)
		}
	}
}
