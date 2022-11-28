package membership

import "context"

type MembershipChange uint16

const (
	MemberAdded MembershipChange = iota
	MemberRemoved
	MemberPinged
)

type MembershipEvent struct {
	ID     string
	Host   string
	Port   uint16
	Change MembershipChange
}

// Provider implements membership methods for a given provider
type Provider interface {
	// Listen returns a channel of membership change events
	Listen(ctx context.Context) (chan MembershipEvent, error)
	// Stop this provider
	Stop(ctx context.Context)
}
