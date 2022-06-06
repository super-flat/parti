package discovery

// Discovery will be implemented by any Service discovery
type Discovery interface {
	// Start starts the discovery service
	Start(nodeID string, nodePort int) (chan string, error)
	// NodeAutoRemovalEnabled indicates whether the actual discovery method supports the automatic node removal or not
	NodeAutoRemovalEnabled() bool
	// Stop should stop the discovery method and all of its goroutines, it should close discovery channel returned in Start
	Stop()
}
