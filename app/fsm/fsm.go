package fsm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	sm "github.com/lni/dragonboat/v3/statemachine"
)

const (
	oneKey = "key"
)

type Query struct {
	Key string
}

type Entry struct {
	Key   string `json:"key"`
	Value string `json:"val"`
}

// ExampleStateMachine is the IStateMachine implementation used in the
// helloworld example.
// See https://github.com/lni/dragonboat/blob/master/statemachine/rsm.go for
// more details of the IStateMachine interface.
type ExampleStateMachine struct {
	ClusterID uint64
	NodeID    uint64
	Count     uint64
	Payload   map[string]*Entry
}

// NewExampleStateMachine creates and return a new ExampleStateMachine object.
func NewExampleStateMachine(clusterID uint64,
	nodeID uint64) sm.IStateMachine {
	return &ExampleStateMachine{
		ClusterID: clusterID,
		NodeID:    nodeID,
		Count:     0,
		Payload:   make(map[string]*Entry),
	}
}

// Lookup performs local lookup on the ExampleStateMachine instance. In this example,
// we always return the Count value as a little endian binary encoded byte
// slice.
func (s *ExampleStateMachine) Lookup(query interface{}) (interface{}, error) {
	result, ok := s.Payload[oneKey]
	if !ok {
		return nil, errors.New("not found")
	}
	output := *result
	return output, nil
}

// Update updates the object using the specified committed raft entry.
func (s *ExampleStateMachine) Update(data []byte) (sm.Result, error) {
	key := oneKey
	value := string(data)
	fmt.Printf("from ExampleStateMachine.Update(), msg: %s, count:%d\n", value, s.Count)
	s.Payload[oneKey] = &Entry{Key: key, Value: value}
	s.Count++
	return sm.Result{Value: uint64(len(data)), Data: data}, nil
}

// SaveSnapshot saves the current IStateMachine state into a snapshot using the
// specified io.Writer object.
func (s *ExampleStateMachine) SaveSnapshot(w io.Writer,
	fc sm.ISnapshotFileCollection, done <-chan struct{}) error {
	// as shown above, the only state that can be saved is the Count variable
	// there is no external file in this IStateMachine example, we thus leave
	// the fc untouched
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, s.Count)
	_, err := w.Write(data)
	return err
}

// RecoverFromSnapshot recovers the state using the provided snapshot.
func (s *ExampleStateMachine) RecoverFromSnapshot(r io.Reader,
	files []sm.SnapshotFile,
	done <-chan struct{}) error {
	// restore the Count variable, that is the only state we maintain in this
	// example, the input files is expected to be empty
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	v := binary.LittleEndian.Uint64(data)
	s.Count = v
	return nil
}

// Close closes the IStateMachine instance. There is nothing for us to cleanup
// or release as this is a pure in memory data store. Note that the Close
// method is not guaranteed to be called as node can crash at any time.
func (s *ExampleStateMachine) Close() error { return nil }
