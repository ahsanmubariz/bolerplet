package kernel

import (
	"fmt"
	"net"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"

	"github.com/ahsanmubariz/bolerplet/framework/config"
)

// Locks in R-004 for our own wiring (NewApp + config.Load). app.Test always
// reports peer 0.0.0.0, so requests go through app.Handler() with a chosen peer.

type hdr struct{ k, v string }

func proxyApp(t *testing.T, proxies string) *fiber.App {
	t.Helper()
	cfg, err := config.Load(func(k string) (string, bool) {
		v, ok := map[string]string{"APP_ENV": "test", "TRUSTED_PROXIES": proxies}[k]
		return v, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp(cfg)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(fmt.Sprintf("%s %s", c.IP(), c.Scheme()))
	})
	return app
}

func serve(app *fiber.App, peer string, hs ...hdr) string {
	var req fasthttp.Request
	req.Header.SetMethod(fiber.MethodGet)
	req.SetRequestURI("/")
	req.Header.SetHost("app.example")
	for _, h := range hs {
		req.Header.Add(h.k, h.v)
	}
	var fctx fasthttp.RequestCtx
	fctx.Init(&req, &net.TCPAddr{IP: net.ParseIP(peer), Port: 40000}, nil)
	app.Handler()(&fctx)
	return string(fctx.Response.Body())
}

func TestProxyTrustIP(t *testing.T) {
	withProxy := proxyApp(t, "10.0.0.0/8, 2001:DB8::1")
	noProxy := proxyApp(t, "")
	xff := func(v string) hdr { return hdr{"X-Forwarded-For", v} }

	tests := []struct {
		name string
		app  *fiber.App
		peer string
		hs   []hdr
		want string
	}{
		{"untrusted peer: XFF ignored", withProxy, "203.0.113.9", []hdr{xff("1.1.1.1")}, "203.0.113.9"},
		{"no proxies configured: XFF ignored", noProxy, "10.0.0.5", []hdr{xff("1.1.1.1")}, "10.0.0.5"},
		{"trusted peer: rightmost untrusted", withProxy, "10.0.0.5", []hdr{xff("6.6.6.6, 203.0.113.7")}, "203.0.113.7"},
		{"trusted peer: invalid entries skipped", withProxy, "10.0.0.5", []hdr{xff("<script>, 203.0.113.7, evil")}, "203.0.113.7"},
		{"trusted peer: garbage only -> peer", withProxy, "10.0.0.5", []hdr{xff("garbage")}, "10.0.0.5"},
		{"trusted peer: repeated lines joined", withProxy, "10.0.0.5", []hdr{xff("6.6.6.6"), xff("203.0.113.7")}, "203.0.113.7"},
		{"non-canonical IPv6 proxy entry still trusted", withProxy, "2001:db8::1", []hdr{xff("203.0.113.7")}, "203.0.113.7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serve(tt.app, tt.peer, tt.hs...); got != tt.want+" http" {
				t.Fatalf("got %q, want ip %s", got, tt.want)
			}
		})
	}
}

func TestProxyTrustScheme(t *testing.T) {
	app := proxyApp(t, "10.0.0.0/8")
	tests := []struct {
		name, peer string
		hs         []hdr
		want       string
	}{
		{"trusted XFP https", "10.0.0.5", []hdr{{"X-Forwarded-Proto", "https"}}, "https"},
		{"untrusted XFP https ignored", "203.0.113.9", []hdr{{"X-Forwarded-Proto", "https"}}, "http"},
		{"untrusted X-Forwarded-Ssl ignored", "203.0.113.9", []hdr{{"X-Forwarded-Ssl", "on"}}, "http"},
		{"untrusted X-Url-Scheme ignored", "203.0.113.9", []hdr{{"X-Url-Scheme", "https"}}, "http"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serve(app, tt.peer, tt.hs...); got != tt.peer+" "+tt.want {
				t.Fatalf("got %q, want scheme %s", got, tt.want)
			}
		})
	}
}
