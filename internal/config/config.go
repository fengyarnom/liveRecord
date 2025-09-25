package config

import (
	"errors"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Site     SiteConfig     `yaml:"site"`
	Security SecurityConfig `yaml:"security"`
}

type ServerConfig struct {
	Port    string `yaml:"port"`
	GinMode string `yaml:"gin_mode"`
}

type DatabaseConfig struct {
	URL string `yaml:"url"`
}

type SiteConfig struct {
	Title       string `yaml:"title"`
	BaseURL     string `yaml:"base_url"`
	Description string `yaml:"description"`
}

type SecurityConfig struct {
	SessionSecret string `yaml:"session_secret"`
}

func Load() (*Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "config.yaml"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	overlayEnv(&c)
	// Normalize and validate site.base_url so generated links are absolute
	c.Site.BaseURL = strings.TrimSpace(c.Site.BaseURL)
	if c.Site.BaseURL == "" || !(strings.HasPrefix(strings.ToLower(c.Site.BaseURL), "http://") || strings.HasPrefix(strings.ToLower(c.Site.BaseURL), "https://")) {
		return nil, errors.New("site.base_url must be an absolute URL with scheme, e.g., https://yarnom.com")
	}
	if c.Server.Port == "" {
		c.Server.Port = "8080"
	}
	if c.Server.GinMode == "" {
		c.Server.GinMode = "release"
	}
	if c.Database.URL == "" {
		return nil, errors.New("database.url is required")
	}
	if c.Security.SessionSecret == "" {
		return nil, errors.New("security.session_secret is required")
	}
	return &c, nil
}

func overlayEnv(c *Config) {
	if v := os.Getenv("PORT"); v != "" {
		c.Server.Port = v
	}
	if v := os.Getenv("GIN_MODE"); v != "" {
		c.Server.GinMode = v
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		c.Database.URL = v
	}
	if v := os.Getenv("SESSION_SECRET"); v != "" {
		c.Security.SessionSecret = v
	}
	c.Server.GinMode = strings.ToLower(c.Server.GinMode)
}
