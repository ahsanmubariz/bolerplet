// Package config loads typed configuration from the environment and enforces
// the production guards of BASELINE §6.13.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

type Env string

const (
	Development Env = "development"
	Test        Env = "test"
	Production  Env = "production"
)

type Config struct {
	Env                  Env
	Addr                 string
	BaseURL              *url.URL
	Debug                bool
	CookieSecure         bool
	CookieEncKey         []byte
	HSTSMaxAge           int
	TrustedProxies       []string // IPs or CIDRs, already validated
	TLSCertFile          string
	TLSKeyFile           string
	DatabaseURL          string
	DBAllowInsecureLocal bool
}

func (c Config) IsProduction() bool { return c.Env == Production }

// TLSInProcess reports whether the server terminates TLS itself.
func (c Config) TLSInProcess() bool { return c.TLSCertFile != "" }

// Load reads the configuration through lookup (os.LookupEnv in production)
// and returns every parse error and guard violation at once.
// Error messages name the variable but never echo its value.
func Load(lookup func(string) (string, bool)) (Config, error) {
	var errs []error
	get := func(key, def string) string {
		if v, ok := lookup(key); ok {
			return strings.TrimSpace(v)
		}
		return def
	}
	getBool := func(key string, def bool) bool {
		v := get(key, "")
		if v == "" {
			return def
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: must be true or false", key))
			return def
		}
		return b
	}

	// Fail closed: an unset APP_ENV gets the production guards.
	c := Config{
		Env:                  Env(get("APP_ENV", string(Production))),
		Addr:                 get("ADDR", ":8080"),
		Debug:                getBool("DEBUG", false),
		CookieSecure:         getBool("COOKIE_SECURE", true),
		TLSCertFile:          get("TLS_CERT_FILE", ""),
		TLSKeyFile:           get("TLS_KEY_FILE", ""),
		DatabaseURL:          get("DATABASE_URL", ""),
		DBAllowInsecureLocal: getBool("DB_ALLOW_INSECURE_LOCAL", false),
	}

	switch c.Env {
	case Development, Test, Production:
	default:
		errs = append(errs, errors.New("APP_ENV: must be development, test or production"))
	}

	defBase := ""
	if !c.IsProduction() {
		defBase = "http://localhost:8080"
	}
	if raw := get("BASE_URL", defBase); raw == "" {
		errs = append(errs, errors.New("BASE_URL: required"))
	} else if u, err := url.Parse(raw); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		errs = append(errs, errors.New("BASE_URL: must be an absolute http(s) URL"))
	} else {
		c.BaseURL = u
	}

	hsts, err := strconv.Atoi(get("HSTS_MAX_AGE", "63072000"))
	if err != nil {
		errs = append(errs, errors.New("HSTS_MAX_AGE: must be an integer"))
	}
	c.HSTSMaxAge = hsts

	if raw := get("COOKIE_ENC_KEY", ""); raw != "" {
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(key) != 32 {
			errs = append(errs, errors.New("COOKIE_ENC_KEY: must be base64 of exactly 32 bytes"))
		} else {
			c.CookieEncKey = key
		}
	}

	for _, p := range strings.Split(get("TRUSTED_PROXIES", ""), ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		if err := checkProxy(p); err != nil {
			errs = append(errs, fmt.Errorf("TRUSTED_PROXIES: entry %d: %w", len(c.TrustedProxies)+1, err))
			continue
		}
		c.TrustedProxies = append(c.TrustedProxies, p)
	}

	if (c.TLSCertFile == "") != (c.TLSKeyFile == "") {
		errs = append(errs, errors.New("TLS_CERT_FILE and TLS_KEY_FILE: set both or neither"))
	}

	errs = append(errs, c.validate()...)
	return c, errors.Join(errs...)
}

// checkProxy accepts a single IP or a CIDR. Fiber silently drops entries it
// cannot parse, so we fail boot instead. A zero-length prefix trusts everyone.
func checkProxy(p string) error {
	if strings.Contains(p, "/") {
		pfx, err := netip.ParsePrefix(p)
		if err != nil {
			return errors.New("not a valid CIDR")
		}
		if pfx.Bits() == 0 {
			return errors.New("must not trust every address (/0)")
		}
		return nil
	}
	if _, err := netip.ParseAddr(p); err != nil {
		return errors.New("not a valid IP or CIDR")
	}
	return nil
}

// validate holds the production-only guards (BASELINE §6.13).
func (c Config) validate() []error {
	if !c.IsProduction() {
		return nil
	}
	var errs []error
	if !c.CookieSecure {
		errs = append(errs, errors.New("COOKIE_SECURE: must be true in production"))
	}
	if c.CookieEncKey == nil {
		errs = append(errs, errors.New("COOKIE_ENC_KEY: required in production"))
	}
	if c.HSTSMaxAge <= 0 {
		errs = append(errs, errors.New("HSTS_MAX_AGE: must be > 0 in production"))
	}
	if !c.TLSInProcess() && len(c.TrustedProxies) == 0 {
		errs = append(errs, errors.New("TRUSTED_PROXIES or TLS_CERT_FILE: required in production, otherwise the scheme is never https and HSTS is never sent"))
	}
	if c.Debug {
		errs = append(errs, errors.New("DEBUG: must be false in production"))
	}
	if c.BaseURL != nil && c.BaseURL.Scheme != "https" {
		errs = append(errs, errors.New("BASE_URL: must be https in production"))
	}
	if err := checkDatabaseURL(c.DatabaseURL, c.DBAllowInsecureLocal); err != nil {
		errs = append(errs, err)
	}
	return errs
}

// checkDatabaseURL requires a postgres:// URL with sslmode verify-full or
// verify-ca. Plain or "require" TLS is allowed only over a Unix socket and
// only with DB_ALLOW_INSECURE_LOCAL=true.
func checkDatabaseURL(raw string, allowInsecureLocal bool) error {
	if raw == "" {
		return errors.New("DATABASE_URL: required in production")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		// Never wrap the url error: it would echo the password.
		return errors.New("DATABASE_URL: must be a postgres:// URL")
	}
	q := u.Query()
	switch q.Get("sslmode") {
	case "verify-full", "verify-ca":
		return nil
	}
	if allowInsecureLocal && u.Host == "" && strings.HasPrefix(q.Get("host"), "/") {
		return nil
	}
	return errors.New("DATABASE_URL: sslmode must be verify-full or verify-ca in production (insecure modes only over a Unix socket with DB_ALLOW_INSECURE_LOCAL=true)")
}
