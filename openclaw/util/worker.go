package util

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

type IBackgroundService interface {
	Name() string
	Execute(ctx context.Context) error
}

type HostManager struct {
	services   []IBackgroundService
	logger     *slog.Logger
	httpServer *http.Server
}

func NewHostManager(logger *slog.Logger) *HostManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &HostManager{logger: logger}
}

func (h *HostManager) AddService(svc IBackgroundService) *HostManager {
	h.services = append(h.services, svc)
	return h
}

func (h *HostManager) SetWebServer(server *http.Server) *HostManager {
	h.httpServer = server
	return h
}

func (h *HostManager) Run() error {
	mainCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(mainCtx)

	// start all registered background services concurrently
	for _, svc := range h.services {
		s := svc
		g.Go(func() error {
			h.logger.Info("--> starting background service", "name", s.Name())
			return s.Execute(ctx)
		})
	}

	// start web server
	if h.httpServer != nil {
		g.Go(func() error {
			return h.runWebServer(ctx)
		})
	}

	h.logger.Info("--> [Host] The application is ready and running...")

	return g.Wait()
}

func (h *HostManager) runWebServer(ctx context.Context) error {
	serverErr := make(chan error, 1)

	go func() {
		h.logger.Info("--> [Host] Start the web service and listen on the port", "address", h.httpServer.Addr)
		if err := h.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		h.logger.Error("web server error", "error", err.Error())
		return fmt.Errorf("web api error: %w", err)
	case <-ctx.Done():
		fmt.Println("--> [Host] closing web server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return h.httpServer.Shutdown(shutdownCtx)
	}
}
