package requestid

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestFromContext(t *testing.T) {
	// create a request ID
	requestID := uuid.NewString()
	ctx := context.Background()
	// set the context with the newly created request ID
	ctx = context.WithValue(ctx, XRequestIDKey{}, requestID)
	// get the request ID
	actual := FromContext(ctx)
	assert.Equal(t, requestID, actual)
}
