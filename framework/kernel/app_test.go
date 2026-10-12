package kernel

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/ahsanmubariz/bolerplet/framework/config"
)

func TestNoServerHeader(t *testing.T) {
	app := NewApp(config.Config{})
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got := resp.Header.Get("Server"); got != "" {
		t.Fatalf("Server header = %q, want none", got)
	}
}

// TestAppConfig locks in the BASELINE §5.1 table.
func TestAppConfig(t *testing.T) {
	t.Run("no proxies", func(t *testing.T) {
		got := NewApp(config.Config{}).Config()
		if got.ServerHeader != "" || got.BodyLimit != 4<<20 || !got.Immutable ||
			got.ReadTimeout != 10*time.Second || got.WriteTimeout != 15*time.Second || got.IdleTimeout != 60*time.Second ||
			got.TrustProxy || got.ProxyHeader != "" || !got.EnableIPValidation || got.SkipUnmatchedRoutes ||
			got.PassLocalsToContext || got.DisableHeadAutoRegister {
			t.Fatalf("unexpected config: %+v", got)
		}
	})
	t.Run("with proxies", func(t *testing.T) {
		got := NewApp(config.Config{TrustedProxies: []string{"10.0.0.0/8"}}).Config()
		tp := got.TrustProxyConfig
		if !got.TrustProxy || got.ProxyHeader != fiber.HeaderXForwardedFor || !got.EnableIPValidation ||
			len(tp.Proxies) != 1 || tp.Loopback || tp.Private || tp.LinkLocal || tp.UnixSocket {
			t.Fatalf("unexpected proxy config: %+v", got)
		}
	})
}

func TestListenConfig(t *testing.T) {
	lc := ListenConfig(context.Background(), config.Config{Env: config.Production})
	if !lc.DisableStartupMessage || lc.TLSMinVersion != tls.VersionTLS12 || lc.ListenerNetwork != fiber.NetworkTCP {
		t.Fatalf("unexpected listen config: %+v", lc)
	}
	if ListenConfig(context.Background(), config.Config{Env: config.Development}).DisableStartupMessage {
		t.Fatal("startup message should show in development")
	}
}

func TestErrorHandlerDoesNotLeak(t *testing.T) {
	app := NewApp(config.Config{})
	app.Get("/boom", func(c fiber.Ctx) error {
		return errors.New(`pq: syntax error at "users" /srv/app/internal/db/x.go:42`)
	})
	app.Get("/teapot", func(c fiber.Ctx) error { return fiber.ErrTeapot })
	for path, want := range map[string]struct {
		code int
		body string
	}{
		"/boom":    {500, "Internal Server Error"},
		"/teapot":  {418, "I'm a teapot"},
		"/missing": {404, "Not Found"},
	} {
		resp, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != want.code || string(b) != want.body {
			t.Errorf("%s: got %d %q, want %d %q", path, resp.StatusCode, b, want.code, want.body)
		}
		if strings.Contains(string(b), "pq:") || strings.Contains(string(b), "/srv/") {
			t.Errorf("%s: body leaks internals: %q", path, b)
		}
	}
}
