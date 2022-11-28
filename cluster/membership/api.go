package membership

import "context"

type ChangeType uint16

const (
	MemberAdded ChangeType = iota
	MemberRemoved
	MemberPinged
)

type Event struct {
	ID     string
	Host   string
	Port   uint16
	Change ChangeType
}

// Provider implements membership methods for a given provider
type Provider interface {
	// Listen returns a channel of membership change events
	Listen(ctx context.Context) (chan Event, error)
	// Stop this provider
	Stop(ctx context.Context)
}
