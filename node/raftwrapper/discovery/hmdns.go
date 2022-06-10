package discovery

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
)

const (
	hmdnsServiceName = "_parti._tcp"
)

type HashicorpDiscovery struct {
	mtx       *sync.Mutex
	isStarted bool
	nodePort  int
	// channel of disovered peers ("host:raftPort")
	discoveryChan chan string
	stopChan      chan bool
}

func NewHashicorpDiscovery(port int) *HashicorpDiscovery {
	return &HashicorpDiscovery{
		mtx:       &sync.Mutex{},
		isStarted: false,
		nodePort:  port,
	}
}

// Start the discovery method and return a channel of node addresses ("host:RaftPort")
func (d *HashicorpDiscovery) Start() (chan string, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	if d.isStarted {
		return nil, errors.New("already started")
	}
	// create the stop channel
	d.stopChan = make(chan bool)
	// create the discovery channel
	d.discoveryChan = make(chan string)
	// run the background process to find peers
	go d.findPeers()
	// return the channel
	d.isStarted = true
	return d.discoveryChan, nil
}

// SupportsNodeAutoRemoval indicates whether the actual discovery method supports the automatic node removal or not
func (d *HashicorpDiscovery) SupportsNodeAutoRemoval() bool {
	return true
}

// Stop should stop the discovery method and all of its goroutines, it should close discovery channel returned in Start
func (d *HashicorpDiscovery) Stop() {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	if !d.isStarted {
		return
	}
	d.stopChan <- true
	d.isStarted = false
}

func (d *HashicorpDiscovery) findPeers() {
	// Setup our service export
	host, _ := os.Hostname()
	info := []string{""}
	service, _ := mdns.NewMDNSService(
		host,
		hmdnsServiceName,
		"",
		"",
		d.nodePort,
		nil,
		info,
	)

	// Create the mDNS server, defer shutdown
	server, _ := mdns.NewServer(&mdns.Config{Zone: service})
	defer server.Shutdown()

	// Make a channel for results and start listening
	entriesCh := make(chan *mdns.ServiceEntry, 4)
	knownClients := make(map[string]string)

	doneCtx, cancel := context.WithCancel(context.Background())

	// first, start reading from the channel
	go func() {
		for {
			select {
			case entry := <-entriesCh:
				addr := fmt.Sprintf("%s:%d", entry.AddrV4, entry.Port)
				if _, found := knownClients[addr]; !found {
					knownClients[addr] = entry.Info
					// log.Printf("Got new entry: addr=%s, info='%s'", addr, entry.Info)
					d.discoveryChan <- addr
				}
			case <-doneCtx.Done():
				return
			}
		}
	}()
	// then, start writing to it
	go func() {
		for {
			select {
			case <-time.After(time.Second * 5):
				mdnsQueryParam := &mdns.QueryParam{
					Service:             hmdnsServiceName,
					Domain:              "local",
					Timeout:             time.Second * 1,
					Entries:             entriesCh,
					WantUnicastResponse: false, // TODO(reddaly): Change this default.
					DisableIPv4:         false,
					DisableIPv6:         true,
				}
				if err := mdns.Query(mdnsQueryParam); err != nil {
					log.Fatal(err)
				}
			case <-doneCtx.Done():
				return
			}
		}
	}()
	// listen for shutdown
	<-d.stopChan
	// kill the cancel context to kill goroutines
	cancel()
}

var _ DiscoveryMethod = &HashicorpDiscovery{}
