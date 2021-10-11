package server

import "github.com/caarlos0/env/v6"

type Config struct {
}

// NewConfigFromEnv instantiates the server config from environment variables
func NewConfigFromEnv() (*Config, error) {
	cfg := &Config{}
	// all env vars are required
	opts := env.Options{RequiredIfNoDef: true}
	if err := env.Parse(cfg, opts); err != nil {
		return nil, err
	}
	return cfg, nil
}
