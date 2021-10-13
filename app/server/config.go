package server

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	RaftNodeID   uint64 `env:"RAFT_ID"`
	RaftPort     string `env:"RAFT_PORT"`
	ApiPort      string `env:"API_PORT"`
	InitialNodes map[int]string
	Peers        []string `env:"PEERS" envDefault:""`
}

// NewConfigFromEnv instantiates the server config from environment variables
func NewConfigFromEnv() (*Config, error) {
	cfg := &Config{}
	// all env vars are required
	opts := env.Options{RequiredIfNoDef: true}
	if err := env.Parse(cfg, opts); err != nil {
		return nil, err
	}
	// parse initial nodes
	if i := os.Getenv("INITIAL_NODES"); i != "" {
		cfg.InitialNodes = make(map[int]string)
		for _, combo := range strings.Split(i, ",") {
			values := strings.Split(combo, "=")
			if len(values) != 2 {
				errMsg := fmt.Sprintf("malformed initial nodes, %s", i)
				return nil, errors.New(errMsg)
			}
			nodeID, err := strconv.Atoi(values[0])
			if err != nil {
				errMsg := fmt.Sprintf("malformed initial nodes, %s", i)
				return nil, errors.New(errMsg)
			}
			nodeAddr := values[1]
			cfg.InitialNodes[nodeID] = nodeAddr
		}
		for k, v := range cfg.InitialNodes {
			fmt.Printf("found node %d at %s\n", k, v)
		}
	}
	return cfg, nil
}
