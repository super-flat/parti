package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/super-flat/raft-poc/app/httpd"
	"github.com/super-flat/raft-poc/app/store"
)

const (
	inmem bool = true
)

func Run(cfg *Config) {
	// Ensure Raft storage exists.
	os.MkdirAll(cfg.RaftDir, 0700)
	// configure raft store
	s := store.New(inmem)
	s.RaftDir = cfg.RaftDir
	s.RaftBind = cfg.RaftAddress
	// boot this node as a "leader" temporarily
	if err := s.Open(cfg.JoinAddress == "", cfg.RaftID); err != nil {
		log.Fatalf("failed to open store: %s", err.Error())
	}
	// create the http server
	h := httpd.New(cfg.NodeAddress, s)
	if err := h.Start(); err != nil {
		log.Fatalf("failed to start HTTP service: %s", err.Error())
	}
	// try to form a cluster
	// time.Sleep(time.Second * 10)
	// broadcastJoin(cfg.PeerAddresses, cfg.RaftAddress, cfg.RaftID)

	// If join address was specified, make the join request to the bootstrap leader
	if cfg.JoinAddress != "" {
		if err := join(cfg.JoinAddress, cfg.RaftAddress, cfg.RaftID); err != nil {
			log.Fatalf("failed to join node at %s: %s", cfg.JoinAddress, err.Error())
		}
	}
	log.Println("hraftd started successfully")
	// await termination
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
	log.Println("hraftd exiting")
}

// func NewRaft(ctx context.Context, fsm raft.FSM, cfg *Config) (*raft.Raft, error) {
// 	return nil, nil
// }

// join will make this node join another node's cluster
func join(joinAddr, raftAddr, nodeID string) error {
	b, err := json.Marshal(map[string]string{"addr": raftAddr, "id": nodeID})
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("http://%s/join", joinAddr), "application-type/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// broadcastJoin will invoke Join for all peers
func broadcastJoin(peers []string, raftAddr string, nodeID string) error {
	for _, peerAddr := range peers {
		err := join(peerAddr, raftAddr, nodeID)
		if err != nil {
			log.Println(err.Error())
			return err
		}
	}
	return nil
}

type BootstrapPeer struct {
	NodeAddress string    `json:"addr"`
	Birthday    time.Time `json:"birthday"`
}

type BootstrapPeers struct {
	Leader string          `json:"leader"`
	Peers  []BootstrapPeer `json:"peers"`
}

type Bootstrapper struct {
	Self  *BootstrapPeer
	Peers map[string]*BootstrapPeer
}

func NewBootstrapper(addr string) *Bootstrapper {
	self := &BootstrapPeer{NodeAddress: addr, Birthday: time.Now()}
	peers := map[string]*BootstrapPeer{}
	peers[self.NodeAddress] = self
	return &Bootstrapper{Self: self, Peers: peers}
}

func (b *Bootstrapper) AddPeers(addresses []string) error {
	for _, peerAddr := range addresses {
		if err := b.AddPeer(peerAddr); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bootstrapper) AddPeer(addr string) error {
	resp, err := http.Get(fmt.Sprintf("http://%s/bootstrap", addr))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body) // response body is []byte
	var respX BootstrapPeers
	if err := json.Unmarshal(body, &respX); err != nil {
		return err
	}
	for _, newPeer := range respX.Peers {
		if newPeer.NodeAddress != b.Self.NodeAddress {
			if _, ok := b.Peers[newPeer.NodeAddress]; !ok {
				b.Peers[newPeer.NodeAddress] = &newPeer
			}
		}
	}
	return nil
}

// func (b *Bootstrapper) Exchange() error {
// 	for _, peer := range b.Peers {
// 		resp, err := http.Get(fmt.Sprintf("http://%s/peers", peer.NodeAddress))
// 		if err != nil {
// 			return err
// 		}
// 		defer resp.Body.Close()
// 		body, err := ioutil.ReadAll(resp.Body) // response body is []byte
// 		var respX BootstrapPeers
// 		if err := json.Unmarshal(body, &respX); err != nil {
// 			return err
// 		}
// 		for _, newPeer := range respX.peers {
// 			b.
// 		}

// 	}
// }

// func (b *Bootstrapper) GetPeers()
