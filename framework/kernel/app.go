// Package kernel builds the Fiber app and (from P2) owns modules and routing.
package kernel

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/ahsanmubariz/bolerplet/framework/config"
)

// BodyLimit is the largest accepted request body (the biggest upload).
const BodyLimit = 4 << 20

// NewApp returns a Fiber app with the BASELINE §5.1 settings (verified in R-003/R-004).
func NewApp(cfg config.Config) *fiber.App {
	proxyHeader := ""
	if len(cfg.TrustedProxies) > 0 {
		proxyHeader = fiber.HeaderXForwardedFor
	}
	return fiber.New(fiber.Config{
		ServerHeader:     "",
		BodyLimit:        BodyLimit,
		ReadTimeout:      10 * time.Second,
		WriteTimeout:     15 * time.Second,
		IdleTimeout:      60 * time.Second,
		Immutable:        true,
		ErrorHandler:     errorHandler,
		TrustProxy:       len(cfg.TrustedProxies) > 0,
		TrustProxyConfig: fiber.TrustProxyConfig{Proxies: cfg.TrustedProxies},
		ProxyHeader:      proxyHeader,
		// MUST stay true: without it c.IP() returns the raw X-Forwarded-For header.
		EnableIPValidation:  true,
		SkipUnmatchedRoutes: false,
	})
}

// ListenConfig returns the listener settings. Cancelling ctx shuts the server
// down gracefully.
func ListenConfig(ctx context.Context, cfg config.Config) fiber.ListenConfig {
	return fiber.ListenConfig{
		GracefulContext:       ctx,
		ListenerNetwork:       fiber.NetworkTCP, // tcp4 is the default and skips IPv6
		DisableStartupMessage: cfg.IsProduction(),
		CertFile:              cfg.TLSCertFile,
		CertKeyFile:           cfg.TLSKeyFile,
		TLSMinVersion:         tls.VersionTLS12,
	}
}

// ponytail: P1 stand-in so Fiber's DefaultErrorHandler (which sends err.Error()
// to the client) never runs. Replaced by httpx.ErrorHandler in P2.
func errorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	var fe *fiber.Error
	if errors.As(err, &fe) {
		code = fe.Code
	}
	if code >= fiber.StatusInternalServerError {
		slog.ErrorContext(c.Context(), "request failed", "status", code, "err", err)
	}
	c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
	return c.Status(code).SendString(http.StatusText(code))
}
