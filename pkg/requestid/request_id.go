package requestid

import (
	"context"
)

// XRequestIDKey is used to store the x-request-id
// into the grpc context
type XRequestIDKey struct{}

const (
	XRequestIDMetadataKey = "x-request-id"
)

// FromContext return the request ID set in context
func FromContext(ctx context.Context) string {
	id, ok := ctx.Value(XRequestIDKey{}).(string)
	if !ok {
		return ""
	}
	return id
}
