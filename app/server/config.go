package server

import "github.com/caarlos0/env/v6"

type Config struct {
	NodeID  int    `env:"NODE_ID" envDefault:"1"`
	Address string `env:"ADDR" envDefault:""`
	Join    bool   `env:"JOIN" envDefault:"false"`
	WebPort string `env:"WEB_PORT" envDefault:"11000"`
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
