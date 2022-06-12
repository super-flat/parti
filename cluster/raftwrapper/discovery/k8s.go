package discovery

type KubernetesDiscovery struct{}

// Start the discovery method, which returns a channel that notifies of
// new nodes discovered (format "IP:RaftPort")
func (k *KubernetesDiscovery) Start() (chan string, error) {
	return nil, nil
}

// SupportsNodeAutoRemoval indicates whether the actual discovery method supports the automatic node removal or not
func (k *KubernetesDiscovery) SupportsNodeAutoRemoval() bool {
	return true
}

// Stop should stop the discovery method and all of its goroutines, it should close discovery channel returned in Start
func (k *KubernetesDiscovery) Stop() {

}

var _ Discovery = &KubernetesDiscovery{}
