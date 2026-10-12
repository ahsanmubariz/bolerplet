package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

var validKey = base64.StdEncoding.EncodeToString(make([]byte, 32))

func lookup(env map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	}
}

func validProd() map[string]string {
	return map[string]string{
		"APP_ENV":         "production",
		"BASE_URL":        "https://app.example.com",
		"COOKIE_ENC_KEY":  validKey,
		"TRUSTED_PROXIES": "10.0.0.0/8",
		"DATABASE_URL":    "postgres://app@db.internal:5432/app?sslmode=verify-full",
	}
}

func TestProdGuards(t *testing.T) {
	if _, err := Load(lookup(validProd())); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}

	tests := []struct {
		name    string
		set     map[string]string
		unset   []string
		wantErr string
	}{
		{"cookie secure off", map[string]string{"COOKIE_SECURE": "false"}, nil, "COOKIE_SECURE"},
		{"enc key missing", nil, []string{"COOKIE_ENC_KEY"}, "COOKIE_ENC_KEY"},
		{"enc key wrong length", map[string]string{"COOKIE_ENC_KEY": base64.StdEncoding.EncodeToString(make([]byte, 16))}, nil, "COOKIE_ENC_KEY"},
		{"enc key not base64", map[string]string{"COOKIE_ENC_KEY": "not base64!"}, nil, "COOKIE_ENC_KEY"},
		{"hsts disabled", map[string]string{"HSTS_MAX_AGE": "0"}, nil, "HSTS_MAX_AGE"},
		{"no tls and no proxy", nil, []string{"TRUSTED_PROXIES"}, "TRUSTED_PROXIES"},
		{"debug on", map[string]string{"DEBUG": "true"}, nil, "DEBUG"},
		{"proxy all ipv4", map[string]string{"TRUSTED_PROXIES": "0.0.0.0/0"}, nil, "TRUSTED_PROXIES"},
		{"proxy all ipv6", map[string]string{"TRUSTED_PROXIES": "10.0.0.1,::/0"}, nil, "TRUSTED_PROXIES"},
		{"db url missing", nil, []string{"DATABASE_URL"}, "DATABASE_URL"},
		{"db sslmode missing", map[string]string{"DATABASE_URL": "postgres://app@db/app"}, nil, "sslmode"},
		{"db sslmode require", map[string]string{"DATABASE_URL": "postgres://app@db/app?sslmode=require"}, nil, "sslmode"},
		{"db insecure local over tcp", map[string]string{"DATABASE_URL": "postgres://app@db/app?sslmode=disable", "DB_ALLOW_INSECURE_LOCAL": "true"}, nil, "sslmode"},
		{"db keyword dsn", map[string]string{"DATABASE_URL": "host=db sslmode=verify-full"}, nil, "DATABASE_URL"},
		{"base url http", map[string]string{"BASE_URL": "http://app.example.com"}, nil, "BASE_URL"},
		{"base url missing", nil, []string{"BASE_URL"}, "BASE_URL"},
		{"tls half configured", map[string]string{"TLS_CERT_FILE": "/c.pem"}, nil, "TLS_KEY_FILE"},
		{"unknown env", map[string]string{"APP_ENV": "prod"}, nil, "APP_ENV"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validProd()
			for k, v := range tt.set {
				env[k] = v
			}
			for _, k := range tt.unset {
				delete(env, k)
			}
			_, err := Load(lookup(env))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want error mentioning %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestProdGuardsAllowed(t *testing.T) {
	tests := []struct {
		name string
		set  map[string]string
	}{
		{"tls in-process instead of proxy", map[string]string{"TRUSTED_PROXIES": "", "TLS_CERT_FILE": "/c.pem", "TLS_KEY_FILE": "/k.pem"}},
		{"verify-ca", map[string]string{"DATABASE_URL": "postgresql://app@db/app?sslmode=verify-ca"}},
		{"insecure local over unix socket", map[string]string{"DATABASE_URL": "postgres:///app?host=/var/run/postgresql&sslmode=disable", "DB_ALLOW_INSECURE_LOCAL": "true"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validProd()
			for k, v := range tt.set {
				env[k] = v
			}
			if _, err := Load(lookup(env)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestProdGuardsReportAllViolations(t *testing.T) {
	_, err := Load(lookup(map[string]string{"APP_ENV": "production", "DEBUG": "true"}))
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"DEBUG", "COOKIE_ENC_KEY", "BASE_URL", "DATABASE_URL", "TRUSTED_PROXIES"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %s: %v", want, err)
		}
	}
}

func TestDefaultEnvIsProduction(t *testing.T) {
	// Fail closed: an unset APP_ENV must get the production guards.
	if _, err := Load(lookup(map[string]string{})); err == nil {
		t.Fatal("empty environment must fail the production guards")
	}
}

func TestGuardsInEveryEnv(t *testing.T) {
	tests := []struct {
		name, key, value string
	}{
		{"unparsable proxy", "TRUSTED_PROXIES", "10.0.0.0/8,not-an-ip"},
		{"zero-length prefix", "TRUSTED_PROXIES", "192.168.0.0/0"},
		{"bad enc key", "COOKIE_ENC_KEY", "short"},
		{"bad bool", "DEBUG", "yes please"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{"APP_ENV": "development", tt.key: tt.value}
			_, err := Load(lookup(env))
			if err == nil || !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("want error mentioning %s, got %v", tt.key, err)
			}
		})
	}
}

func TestDevelopmentDefaults(t *testing.T) {
	cfg, err := Load(lookup(map[string]string{"APP_ENV": "development", "TRUSTED_PROXIES": " 10.0.0.1 , 172.16.0.0/12 , 2001:DB8:0::1, ::ffff:10.0.0.2, 10.1.2.3/8"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" || !cfg.CookieSecure || cfg.HSTSMaxAge != 63072000 || cfg.BaseURL.String() != "http://localhost:8080" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if got := strings.Join(cfg.TrustedProxies, ","); got != "10.0.0.1,172.16.0.0/12,2001:db8::1,10.0.0.2,10.0.0.0/8" {
		t.Fatalf("proxies = %q", got)
	}
}

func TestErrorsNeverEchoSecrets(t *testing.T) {
	env := validProd()
	env["DATABASE_URL"] = "postgres://app:hunter2@db/app?sslmode=disable"
	env["COOKIE_ENC_KEY"] = "c2VjcmV0LXNob3J0"
	_, err := Load(lookup(env))
	if err == nil {
		t.Fatal("expected errors")
	}
	if strings.Contains(err.Error(), "hunter2") || strings.Contains(err.Error(), "c2VjcmV0LXNob3J0") {
		t.Fatalf("error leaks a secret: %v", err)
	}
}
