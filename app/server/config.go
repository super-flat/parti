package server

import "github.com/caarlos0/env/v6"

type Config struct {
	NodeAddress string `env:"NODE_ADDRESS" envDefault:"localhost:11000"`
	RaftAddress string `env:"RAFT_ADDRESS" envDefault:"localhost:12000"`
	RaftID      string `env:"RAFT_ID"`
	RaftDir     string `env:"RAFT_DIR" envDefault:"data/"`
	// shouldBootstrap bool   `env:"RAFT_BOOTSTRAP" envDefault:"false"`
	JoinAddress string `env:"JOIN_ADDRESS"`
	// PeerAddresses []string `env:"PEER_ADDRESSES"`
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
