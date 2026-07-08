// Package common provides shared configuration, logging, and model types.
package common

import (
	"fmt"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	Enabled bool   `yaml:"enabled"`
}

type GatewayConfig struct {
	Port int    `yaml:"port"`
	Name string `yaml:"name"`
}

type MongoConfig struct {
	URI      string `yaml:"uri"`
	Database string `yaml:"database"`
}

type Config struct {
	Gateway GatewayConfig   `yaml:"gateway"`
	MongoDB MongoConfig     `yaml:"mongodb"`
	Servers []ServerConfig  `yaml:"servers"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if envURI := os.Getenv("MONGO_URI"); envURI != "" {
		cfg.MongoDB.URI = envURI
	}
	if envDB := os.Getenv("MONGO_DATABASE"); envDB != "" {
		cfg.MongoDB.Database = envDB
	}

	if cfg.Gateway.Port == 0 {
		cfg.Gateway.Port = 8080
	}

	seen := make(map[string]bool)
	for _, s := range cfg.Servers {
		if s.Name == "" {
			return nil, fmt.Errorf("server with empty name")
		}
		if seen[s.Name] {
			return nil, fmt.Errorf("duplicate server %q", s.Name)
		}
		seen[s.Name] = true
		if s.Enabled && s.URL == "" {
			return nil, fmt.Errorf("server %q enabled but no URL", s.Name)
		}
		if s.URL != "" {
			if _, err := url.ParseRequestURI(s.URL); err != nil {
				return nil, fmt.Errorf("server %q invalid URL: %w", s.Name, err)
			}
		}
	}
	return &cfg, nil
}
