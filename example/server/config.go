package server

import "github.com/caarlos0/env/v6"

type Config struct {
	HttpPort      uint16 `env:"HTTP_PORT" envDefault:"50000"`
	RaftPort      uint16 `env:"RAFT_PORT" envDefault:"50100"`
	DiscoveryPort uint16 `env:"DISCOVERY_PORT" envDefault:"50200"`
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
