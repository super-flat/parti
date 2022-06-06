package discovery

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/pkg/errors"
	"github.com/super-flat/parti/logging"
)

const (
	mdnsServiceName = "_parti._tcp"
)

// MDNSDiscovery uses Multicast DNS method to discover services on the local network without the use of an authoritative DNS server.
// This enables peer-to-peer discovery
type MDNSDiscovery struct {
	delayTime     time.Duration
	nodeID        string
	nodePort      int
	discoveryChan chan string
	stopChan      chan bool
	mdnsServer    *zeroconf.Server
	mdnsResolver  *zeroconf.Resolver
}

var _ Discovery = (*MDNSDiscovery)(nil)

// NewMDNSDiscovery creates an instance of MDNSDiscovery
func NewMDNSDiscovery() *MDNSDiscovery {
	// create the delay time
	rand.Seed(time.Now().UnixNano())
	delayTime := time.Duration(rand.Intn(5)+1) * time.Second
	// create an instance of MDNSDiscovery
	return &MDNSDiscovery{
		delayTime:     delayTime,
		discoveryChan: make(chan string),
		stopChan:      make(chan bool),
	}
}

// Start starts the MDNS discovery service
func (d *MDNSDiscovery) Start(nodeID string, nodePort int) (chan string, error) {
	// set the fields' values
	d.nodeID = nodeID
	d.nodePort = nodePort
	// register service
	srv, err := zeroconf.
		Register(d.nodeID,
			mdnsServiceName,
			"local.",
			d.nodePort,
			[]string{"txtv=0", "lo=1", "la=2"},
			nil)
	// handle the error
	if err != nil {
		return nil, err
	}

	// fetch mDNS enabled raft nodes
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		// log the error and panic
		// TODO to be discussed
		logging.Panic(errors.Wrap(err, "unable to initialize mDNS resolver"))
	}

	// set the MDNS server and resolver
	d.mdnsServer = srv
	d.mdnsResolver = resolver
	// perform service lookup
	go d.discover()
	return d.discoveryChan, nil
}

// NodeAutoRemovalEnabled specifies whether automatic node removal is enabled
func (d *MDNSDiscovery) NodeAutoRemovalEnabled() bool {
	return true
}

// Stop stops the MDNS discovery service
func (d *MDNSDiscovery) Stop() {
	// set the stop channel to true
	d.stopChan <- true
	// shutdown the mdns discovery server
	d.mdnsServer.Shutdown()
	// close the discovery channel
	close(d.discoveryChan)
}

func (d *MDNSDiscovery) discover() {
	// Make a channel for results and start listening
	entriesCh := make(chan *zeroconf.ServiceEntry)
	go func() {
		for {
			select {
			case <-d.stopChan:
				break
			case entry := <-entriesCh:
				d.discoveryChan <- fmt.Sprintf("%s:%d", entry.AddrIPv4[0], entry.Port)
			}
		}
	}()
	// create a cancellation context
	ctx, cancel := context.WithCancel(context.Background())
	for {
		select {
		case <-d.stopChan:
			cancel()
			break
		default:
			err := d.mdnsResolver.Browse(ctx, mdnsServiceName, "local.", entriesCh)
			if err != nil {
				logging.Error("error during mDNS lookup: %v\n", err)
			}
			time.Sleep(d.delayTime)
		}
	}
}
