package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	dbfs "github.com/rifqif16/ecampus/backend/db"
	"github.com/rifqif16/ecampus/backend/internal/platform/config"
	"github.com/rifqif16/ecampus/backend/internal/platform/db"
	"github.com/rifqif16/ecampus/backend/internal/platform/health"
	"github.com/rifqif16/ecampus/backend/internal/platform/logger"
	"github.com/rifqif16/ecampus/backend/internal/platform/server"
)

var errUsage = errors.New("usage: api <serve|migrate>")

type command func(ctx context.Context, cfg config.Config, log *slog.Logger) error

var commands = map[string]command{
	"serve":   serve,
	"migrate": migrate,
}

func main() {
	os.Exit(execute())
}

func execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Getenv, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string, getenv func(string) string, stderr io.Writer) error {
	if len(args) != 1 {
		return errUsage
	}
	cmd, ok := commands[args[0]]
	if !ok {
		return fmt.Errorf("%w (unknown command %q)", errUsage, args[0])
	}

	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	log, err := logger.New(stderr, cfg.LogLevel)
	if err != nil {
		return err
	}
	return cmd(ctx, cfg, log)
}

func serve(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	pool, err := db.NewPool(ctx, cfg.DatabaseURL, db.DefaultPoolOptions())
	if err != nil {
		return err
	}
	defer pool.Close()

	handler := server.NewRouter(server.Deps{
		Log:    log,
		Health: health.NewHandler(log, 0, health.PingCheck("database", pool)),
	})

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.HTTPAddr, err)
	}

	log.Info("api listening", "addr", ln.Addr().String())
	err = server.Run(ctx, server.NewHTTPServer(cfg.HTTPAddr, handler, log), ln, server.DefaultShutdownTimeout)
	log.Info("api stopped")
	return err
}

func migrate(_ context.Context, cfg config.Config, log *slog.Logger) error {
	if err := db.Migrate(cfg.DatabaseURL, dbfs.Migrations, dbfs.MigrationsDir); err != nil {
		return err
	}
	log.Info("migrations applied")
	return nil
}
