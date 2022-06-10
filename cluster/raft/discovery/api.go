package discovery

// IDiscovery gives the interface to perform automatic Node discovery
type IDiscovery interface {
	// Start is about to start the discovery method
	// it returns a channel where the node will consume node addresses ("IP:NodeRaftPort") until the channel gets closed
	Start(nodeID string) (chan string, error)

	// SupportsNodeAutoRemoval indicates whether the actual discovery method supports the automatic node removal or not
	SupportsNodeAutoRemoval() bool

	// Stop should stop the discovery method and all of its goroutines, it should close discovery channel returned in Start
	Stop()
}
