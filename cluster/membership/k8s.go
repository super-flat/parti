package membership

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	partilog "github.com/super-flat/parti/log"
)

type k8sPeer struct {
	ID        string
	Address   string
	Port      uint16
	LastEvent time.Time
	IsLive    bool
}

// Kubernetes implements the membership.Provider interface via direct
// integration with the kubernetes API
type Kubernetes struct {
	logger            partilog.Logger
	namespace         string
	podLabels         map[string]string
	portName          string
	isStarted         bool
	mtx               *sync.Mutex
	discoCh           chan Event
	peerCache         map[string]bool // peer ID -> bool is active
	shutdownCallbacks []func()
	k8sClient         *kubernetes.Clientset
	internalCh        chan Event
}

var _ Provider = &Kubernetes{}

func NewKubernetes(namespace string, podLabels map[string]string, portName string) *Kubernetes {
	// copy pod labels into new map
	podLabelsCopy := make(map[string]string, len(podLabels))
	for k, v := range podLabels {
		podLabels[k] = v
	}
	k := &Kubernetes{
		logger:     partilog.DefaultLogger, // TODO move to a config
		namespace:  namespace,
		podLabels:  podLabelsCopy,
		portName:   portName,
		mtx:        &sync.Mutex{},
		isStarted:  false,
		peerCache:  make(map[string]bool),
		discoCh:    make(chan Event, 10),
		internalCh: make(chan Event, 10),
	}
	return k
}

// GetNodeID returns the pod name set by an environment variable
func (k *Kubernetes) GetNodeID(ctx context.Context) (string, error) {
	nodeID := os.Getenv("POD_NAME")
	if nodeID == "" {
		return "", errors.New("missing POD_NAME env var")
	}
	return nodeID, nil
}

// Listen returns a channel of membership change events
func (k *Kubernetes) Listen(ctx context.Context) (chan Event, error) {
	k.mtx.Lock()
	defer k.mtx.Unlock()
	if k.isStarted {
		return nil, errors.New("already started")
	}
	k.logger.Info("starting k8s membership")

	// make the context cancelable
	runningContext, cancelContext := context.WithCancel(ctx)
	k.shutdownCallbacks = append(k.shutdownCallbacks, cancelContext)

	// create k8s client
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	k.k8sClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// kick off loop in goroutine
	go k.processEvents(runningContext)
	go k.pollPods(runningContext)
	go k.listenChanges(runningContext)

	// report started and return channel
	k.isStarted = true
	return k.discoCh, nil
}

// Stop should stop the discovery method and all of its goroutines, it should close discovery channel returned in Start
func (k *Kubernetes) Stop(ctx context.Context) {
	k.mtx.Lock()
	defer k.mtx.Unlock()
	if k.isStarted {
		k.logger.Info("stopping k8s membership")
		k.isStarted = false
		for _, callback := range k.shutdownCallbacks {
			callback()
		}
		// TODO: mange this carefully b/c we have goroutines publishing to it
		// and dont want a panic publishing to closed channel
		close(k.discoCh)
	}
}

// processEvents reads the internal event stream and publishes to the public
// discovery channel
func (k *Kubernetes) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-k.internalCh:
			k.mtx.Lock()
			switch evt.Change {
			case MemberAdded, MemberPinged:
				isActive, exists := k.peerCache[evt.ID]

				if !exists {
					// if doesn't exist yet
					// set to active
					k.peerCache[evt.ID] = true
					// emit an added event
					k.discoCh <- Event{
						ID:     evt.ID,
						Host:   evt.Host,
						Port:   evt.Port,
						Change: MemberAdded,
					}
				} else if isActive {
					// if it's already active, emit a ping event
					k.discoCh <- Event{
						ID:     evt.ID,
						Host:   evt.Host,
						Port:   evt.Port,
						Change: MemberPinged,
					}
				}

			case MemberRemoved:
				k.peerCache[evt.ID] = false
				k.discoCh <- evt
			}
			k.mtx.Unlock()
		}
	}
}

// listenChanges subscribes to pod chagnes
// TODO: catch the shutdown/terminating much earlier!
func (k *Kubernetes) listenChanges(ctx context.Context) {
	k.logger.Debugf("creating a k8s watcher")
	watchOpts := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(k.podLabels).String(),
	}
	var watcher watch.Interface
	var err error
	for {
		watcher, err = k.k8sClient.CoreV1().Pods(k.namespace).Watch(ctx, watchOpts)
		if err != nil {
			k.logger.Error(err)
			time.Sleep(time.Second)
		} else {
			break
		}
	}
	// consume changes
	for {
		select {
		case <-ctx.Done():
			watcher.Stop()
			return

		case event := <-watcher.ResultChan():
			pod, ok := event.Object.(*v1.Pod)
			if !ok {
				k.logger.Error("unexpected type")
				continue
			}
			k.logger.Debugf("received watch event %s for pod %s", event.Type, pod.Name)

			if pod.Status.PodIP == "" {
				k.logger.Debugf("pod %s does not have an IP yet", pod.Name)
				continue
			}

			if k.isSelf(pod.Status.PodIP) {
				continue
			}

			var newPeer *k8sPeer

			for i := 0; i < len(pod.Spec.Containers) && newPeer == nil; i++ {
				container := pod.Spec.Containers[i]
				for _, port := range container.Ports {
					if port.Name == k.portName {
						// create the peer
						newPeer = &k8sPeer{
							ID:        pod.GetName(),
							Address:   pod.Status.PodIP,
							Port:      uint16(port.ContainerPort),
							LastEvent: time.Now(),
							IsLive:    true,
						}
						break
					}
				}
			}

			if newPeer == nil {
				k.logger.Debugf("pod %s matched selector but did not have port named %s", pod.Name, k.portName)
				continue
			}

			switch event.Type {
			case watch.Added:
				k.internalCh <- Event{
					ID:     newPeer.ID,
					Host:   newPeer.Address,
					Port:   newPeer.Port,
					Change: MemberAdded,
				}

			case watch.Deleted:
				k.internalCh <- Event{
					ID:     newPeer.ID,
					Host:   newPeer.Address,
					Port:   newPeer.Port,
					Change: MemberRemoved,
				}

			case watch.Modified:
				switch pod.Status.Phase {
				case v1.PodRunning:
					k.internalCh <- Event{
						ID:     newPeer.ID,
						Host:   newPeer.Address,
						Port:   newPeer.Port,
						Change: MemberPinged,
					}
				case v1.PodSucceeded, v1.PodFailed:
					k.internalCh <- Event{
						ID:     newPeer.ID,
						Host:   newPeer.Address,
						Port:   newPeer.Port,
						Change: MemberRemoved,
					}

				default:
					// pass
				}

			default:
				k.logger.Debugf("watcher skipping event type %s", event.Type)
			}
		}
	}
}

// pollPods loops over the k8s pods and reports additions and removals
func (k *Kubernetes) pollPods(ctx context.Context) {
	k.logger.Debug("starting pollPods")
	for {
		select {
		case <-ctx.Done():
			k.logger.Debug("shutting down pollPods")
			return
		case <-time.After(time.Second):
			// select pods that have specific labels
			pods, err := k.k8sClient.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
				LabelSelector: labels.SelectorFromSet(k.podLabels).String(),
			})
			if err != nil {
				k.logger.Errorf("could not list pods for kubernetes discovery, %v", err)
			}
			// enumerate pods that match label selector
			for _, pod := range pods.Items {
				// only consider running pods
				if pod.Status.Phase != v1.PodRunning {
					continue
				}
				if k.isSelf(pod.Status.PodIP) {
					continue
				}
				// enumerate containers searching for the port
				var newPeer *k8sPeer
				// search for the named port
				for i := 0; i < len(pod.Spec.Containers) && newPeer == nil; i++ {
					container := pod.Spec.Containers[i]
					for _, port := range container.Ports {
						if port.Name == k.portName {
							// create the peer
							newPeer = &k8sPeer{
								ID:        pod.GetName(),
								Address:   pod.Status.PodIP,
								Port:      uint16(port.ContainerPort),
								LastEvent: time.Now(),
								IsLive:    true,
							}
							break
						}
					}
				}
				if newPeer == nil {
					k.logger.Debugf("pod %s matched selector but did not have port named %s", pod.Name, k.portName)
					continue
				}

				k.internalCh <- Event{
					ID:     newPeer.ID,
					Host:   newPeer.Address,
					Port:   newPeer.Port,
					Change: MemberPinged,
				}
			}
		}
	}
}

var _ Provider = &Kubernetes{}

// isSelf returns true if the given address matches any local addresses
// TODO: this is probably not the best way to do this, make a remote call instead?
func (k *Kubernetes) isSelf(address string) bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		k.logger.Errorf("failed to get addresses, %v", err)
		return false
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			log.Fatalf("failed to read addr, %v", err)
		} else {
			// handle err
			for _, interfaceAddress := range addrs {
				var ip net.IP
				switch v := interfaceAddress.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip.String() == address {
					return true
				}
			}
		}
	}
	return false
}
