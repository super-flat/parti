package discovery

import "context"

type EventType uint16

const (
	MemberAdded EventType = iota
	MemberRemoved
	MemberPinged
)

// Event specifies the discovery event
type Event struct {
	ID   string
	Host string
	Port uint16
	Type EventType
}

// Provider implements discovery methods for a given provider
type Provider interface {
	// GetNodeID returns this node's unique ID in the cluster
	GetNodeID(ctx context.Context) (nodeID string, err error)
	// Listen returns a channel of membership change events
	Listen(ctx context.Context) (chan Event, error)
	// Stop this provider
	Stop(ctx context.Context)
}
