package fsm

import (
	"sync"

	hraft "github.com/hashicorp/raft"
	partipb "github.com/super-flat/parti/pb/parti/v1"
	"go.uber.org/multierr"
	"google.golang.org/protobuf/proto"
)

type snapshot struct {
	sync.Mutex
	groups *partipb.FsmGroups
}

var _ hraft.FSMSnapshot = &snapshot{}

// Persist implementation
func (s *snapshot) Persist(sink hraft.SnapshotSink) error {
	// acquire the lock
	s.Lock()
	// let us marshal the state
	// no need to handle the error because groups is a valid proto message
	bytea, _ := proto.Marshal(s.groups)
	// sync the data data
	_, err := sink.Write(bytea)
	// handle the error
	if err != nil {
		// define a definite error to return
		var combinedErr error
		// cancel the sync
		if e := sink.Cancel(); e != nil {
			// let us combine the errors and return them
			combinedErr = multierr.Combine(err, e)
		}
		// return the combined err if defined
		if combinedErr != nil {
			return combinedErr
		}
		return err
	}
	return sink.Close()
}

// Release implementation
func (s *snapshot) Release() {
	s.Unlock()
}
