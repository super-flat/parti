package discovery

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type KubernetesDiscovery struct {
	namespace string
	podLabels map[string]string
	portName  string
	stopChan  chan bool
	isStarted bool
	mtx       *sync.Mutex
	discoCh   chan string
	cacheTime time.Duration // how long before we re-report a peer with the same address
}

func NewKubernetesDiscovery(namespace string, podLabels map[string]string, portName string) *KubernetesDiscovery {
	// copy pod labels into new map
	podLabelsCopy := make(map[string]string, len(podLabels))
	for k, v := range podLabels {
		podLabels[k] = v
	}
	return &KubernetesDiscovery{
		namespace: namespace,
		podLabels: podLabelsCopy,
		portName:  portName,
		mtx:       &sync.Mutex{},
		stopChan:  make(chan bool),
		cacheTime: time.Minute,
		isStarted: false,
	}
}

// Start the discovery method, which returns a channel that notifies of
// new nodes discovered (format "IP:RaftPort")
func (k *KubernetesDiscovery) Start() (chan string, error) {
	k.mtx.Lock()
	defer k.mtx.Unlock()
	if k.isStarted {
		return nil, errors.New("already started")
	}
	// create k8s client
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	// create discovery channel
	k.discoCh = make(chan string)
	// kick off discovery loop in goroutine, pass in k8s client
	go k.discovery(clientSet)
	// report started and return channel
	k.isStarted = true
	return k.discoCh, nil
}

// SupportsNodeAutoRemoval indicates whether the actual discovery method supports the automatic node removal or not
func (k *KubernetesDiscovery) SupportsNodeAutoRemoval() bool {
	return true
}

// Stop should stop the discovery method and all of its goroutines, it should close discovery channel returned in Start
func (k *KubernetesDiscovery) Stop() {
	k.mtx.Lock()
	defer k.mtx.Unlock()
	if k.isStarted {
		k.stopChan <- true
		k.isStarted = false
		close(k.discoCh)
	}
}

func (k *KubernetesDiscovery) discovery(clientSet *kubernetes.Clientset) {
	// map of active peers and when we reported them
	peerCache := make(map[string]time.Time)

	for {
		select {
		case <-k.stopChan:
			return
		default:
			ctx := context.Background()
			// select pods that have specific labels
			pods, err := clientSet.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
				LabelSelector: labels.SelectorFromSet(k.podLabels).String(),
			})
			if err != nil {
				log.Printf("could not list pods for kubernetes discovery, %v", err)
			}
			// enumerate pods that match label selector
			for _, pod := range pods.Items {
				// only consider running pods
				if pod.Status.Phase == v1.PodRunning {
					// enumerate containers searching for the port
					for _, container := range pod.Spec.Containers {
						for _, port := range container.Ports {
							if port.Name == k.portName {
								peerAddr := fmt.Sprintf("%s:%d", pod.Status.PodIP, port.ContainerPort)
								seenAt, seenBefore := peerCache[peerAddr]
								// if it's new OR seen a while ago, report it to the
								// discovery channel
								if !seenBefore || time.Since(seenAt) > k.cacheTime {
									log.Printf("found k8s peer %s", peerAddr)
									peerCache[peerAddr] = time.Now()
									// lock to prevent panic if discovery channel
									// is shutting down
									if k.mtx.TryLock() && k.isStarted {
										k.discoCh <- peerAddr
										k.mtx.Unlock()
									}
								}
							}
						}
					}
				}
			}
			// sleep until next loop
			// TODO: make configurable?
			time.Sleep(time.Second * 5)
		}
	}
}

var _ Discovery = &KubernetesDiscovery{}
