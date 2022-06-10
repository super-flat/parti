package discovery

// DiscoveryMethod gives the interface to perform automatic Node discovery
type DiscoveryMethod interface {
	// Start the discovery method, which returns a channel that notifies of
	// new nodes discovered (format "IP:RaftPort")
	Start() (chan string, error)

	// SupportsNodeAutoRemoval indicates whether the actual discovery method supports the automatic node removal or not
	SupportsNodeAutoRemoval() bool

	// Stop should stop the discovery method and all of its goroutines, it should close discovery channel returned in Start
	Stop()
}
