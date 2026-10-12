// Command server is the front controller: config → kernel → modules → listen.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v3"

	"github.com/ahsanmubariz/bolerplet/framework/config"
	"github.com/ahsanmubariz/bolerplet/framework/kernel"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	if err := run(); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}

	app := kernel.NewApp(cfg)
	ok := func(c fiber.Ctx) error { return c.SendString("ok") }
	// ponytail: registered on the app directly until kernel.Router exists (P2),
	// then moved behind policy.Public. /readyz gains a DB ping in P4.
	app.Get("/healthz", ok)
	app.Get("/readyz", ok)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return app.Listen(cfg.Addr, kernel.ListenConfig(ctx, cfg))
}
