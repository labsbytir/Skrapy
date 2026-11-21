package proxy

import (
	"fmt"
	"net/url"
	"os"
)

// Config holds proxy credentials. Values come from env; defaults are placeholders.
type Config struct {
	Host     string
	Port     string
	Username string
	Password string
}

func FromEnv() Config {
	c := Config{
		Host:     envOr("PROXY_HOST", "isp.decodo.com"),
		Port:     envOr("PROXY_PORT", "10000"),
		Username: envOr("PROXY_USERNAME", "USERNAME"),
		Password: envOr("PROXY_PASSWORD", "PASSWORD"),
	}
	return c
}

func (c Config) URL() string {
	if c.Username == "USERNAME" || c.Password == "PASSWORD" {
		return ""
	}
	u := &url.URL{
		Scheme: "http",
		User:   url.UserPassword(c.Username, c.Password),
		Host:   fmt.Sprintf("%s:%s", c.Host, c.Port),
	}
	return u.String()
}

// LightpandaArgs returns CLI flags to pass to `lightpanda serve` for this proxy.
func (c Config) LightpandaArgs() []string {
	if pu := c.URL(); pu != "" {
		return []string{"--http-proxy", pu}
	}
	return nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}