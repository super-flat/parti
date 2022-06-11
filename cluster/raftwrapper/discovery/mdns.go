package discovery

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	mdnsServiceName = "_parti._tcp"
)

type MDNSDiscovery struct {
	delayTime     time.Duration
	raftPort      int
	discoveryChan chan string
	stopChan      chan bool
	isStarted     bool
	mtx           *sync.Mutex
}

func NewMDNSDiscovery(raftPort int) *MDNSDiscovery {
	rand.Seed(time.Now().UnixNano())
	delayTime := time.Duration(rand.Intn(5)+1) * time.Second
	return &MDNSDiscovery{
		delayTime:     delayTime,
		discoveryChan: make(chan string),
		stopChan:      make(chan bool),
		isStarted:     false,
		mtx:           &sync.Mutex{},
		raftPort:      raftPort,
	}
}

func (d *MDNSDiscovery) Start() (chan string, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	if d.isStarted {
		return nil, errors.New("already started")
	}
	d.discoveryChan = make(chan string)
	go d.discovery()
	d.isStarted = true
	return d.discoveryChan, nil
}

func (d *MDNSDiscovery) discovery() {
	// expose mdns server
	instance, _ := os.Hostname()
	mdnsServer, err := zeroconf.Register(
		instance,
		mdnsServiceName,
		"local.",
		d.raftPort,
		[]string{"txtv=0", "lo=1", "la=2"},
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer mdnsServer.Shutdown()

	// fetch mDNS enabled raft nodes
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize mDNS resolver:", err.Error())
	}
	// create cache of seen nodes
	seenPeers := make(map[string]time.Time)
	// create channel to listen for new entries
	entries := make(chan *zeroconf.ServiceEntry)
	// make a cancel context to single completion
	ctx, cancel := context.WithCancel(context.Background())
	// create a waitgroup to ensure goroutine shutdown
	wg := &sync.WaitGroup{}
	// start listening in goroutine
	wg.Add(1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				wg.Done()
				return
			case entry := <-entries:
				newPeer := fmt.Sprintf("%s:%d", entry.AddrIPv4[0], entry.Port)
				_, seen := seenPeers[newPeer]
				if !seen {
					log.Printf("found new peer %s", newPeer)
					// only write record if isStarted = true, because
					// stop method closes the channel
					d.mtx.Lock()
					if d.isStarted {
						d.discoveryChan <- newPeer
						seenPeers[newPeer] = time.Now()
					}
					d.mtx.Unlock()
				}
			}
		}
	}()
	// start writing entries in a goroutine
	wg.Add(1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				wg.Done()
				return
			default:
				// create an inner context and entries channel each
				// time we call resolver.Browse because this library closes
				// the provided entries channel internally, so if you run it
				// multiple times, the 2nd run panics trying to close a closed
				// channel...
				innerCtx, innerCancel := context.WithCancel(ctx)
				innerEntries := make(chan *zeroconf.ServiceEntry)
				if err = resolver.Browse(innerCtx, mdnsServiceName, "local.", innerEntries); err != nil {
					log.Printf("failed to write entries, %v", err)
				}
				doLoop := true
				for doLoop {
					select {
					case <-time.After(d.delayTime):
						// if no records received within delayTime, restart
						// the browser
						doLoop = false
					case entry := <-innerEntries:
						entries <- entry
					}
				}
				innerCancel()
			}
		}
	}()
	// wait for shutdown
	<-d.stopChan
	cancel()
	wg.Wait()
}

func (d *MDNSDiscovery) SupportsNodeAutoRemoval() bool {
	return true
}

func (d *MDNSDiscovery) Stop() {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	if d.isStarted {
		d.stopChan <- true
		d.isStarted = false
		close(d.discoveryChan)
	}
}
