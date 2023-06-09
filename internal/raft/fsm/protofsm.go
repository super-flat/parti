package fsm

import (
	"io"
	"log"
	"sync"

	hraft "github.com/hashicorp/raft"
	"github.com/pkg/errors"
	"github.com/super-flat/parti/internal/raft/serializer"
	partipb "github.com/super-flat/parti/pb/parti/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

type ProtoFsm struct {
	data map[string]map[string]proto.Message
	mtx  *sync.Mutex
	ser  *serializer.ProtoSerializer
}

var _ hraft.FSM = &ProtoFsm{}

func NewProtoFsm() *ProtoFsm {
	return &ProtoFsm{
		data: make(map[string]map[string]proto.Message),
		mtx:  &sync.Mutex{},
		ser:  serializer.NewProtoSerializer(),
	}
}

// Apply is called once a log entry is committed by a majority of the cluster.
//
// Apply should apply the log to the FSM. Apply must be deterministic and
// produce the same result on all peers in the cluster.
//
// The returned value is returned to the client as the ApplyFuture.Response.
func (p *ProtoFsm) Apply(raftLog *hraft.Log) interface{} {
	if raftLog == nil || raftLog.Data == nil {
		log.Println("Raft received nil log apply!")
		return nil
	}
	switch raftLog.Type {
	case hraft.LogCommand:
		msg, err := p.ser.DeserializeProto(raftLog.Data)
		if err != nil {
			return err
		}
		result, err := p.applyProtoCommand(msg)
		if err != nil {
			return err
		}
		return result
	default:
		return nil
	}
}

func (p *ProtoFsm) applyProtoCommand(cmd proto.Message) (proto.Message, error) {
	switch v := cmd.(type) {
	case *partipb.FsmGetRequest:
		output, err := p.Get(v.GetGroup(), v.GetKey())
		return output, err

	case *partipb.FsmPutRequest:
		value, err := v.GetValue().UnmarshalNew()
		if err != nil {
			return nil, err
		}
		err = p.put(v.GetGroup(), v.GetKey(), value)
		return nil, err

	case *partipb.FsmRemoveRequest:
		p.remove(v.GetGroup(), v.GetKey())
		return nil, nil

	case *partipb.BulkFsmPutRequest:
		requests := v.GetRequests()
		values := make([]protoreflect.ProtoMessage, len(requests))
		for ix, request := range requests {
			value, err := request.GetValue().UnmarshalNew()
			if err != nil {
				return nil, err
			}
			values[ix] = value
		}
		for ix := 0; ix < len(requests); ix++ {
			if err := p.put(requests[ix].GetGroup(), requests[ix].GetKey(), values[ix]); err != nil {
				return nil, err
			}
		}
		return nil, nil

	default:
		// TODO: decide if this is the right design
		return nil, errors.New("unknown request type")
	}
}

// Snapshot returns an FSMSnapshot used to: support log compaction, to
// restore the FSM to a previous state, or to bring out-of-date followers up
// to a recent log index.
//
// The Snapshot implementation should return quickly, because Apply can not
// be called while Snapshot is running. Generally this means Snapshot should
// only capture a pointer to the state, and any expensive IO should happen
// as part of FSMSnapshot.Persist.
//
// Apply and Snapshot are always called from the same thread, but Apply will
// be called concurrently with FSMSnapshot.Persist. This means the FSM should
// be implemented to allow for concurrent updates while a snapshot is happening.
func (p *ProtoFsm) Snapshot() (hraft.FSMSnapshot, error) {
	// acquire the lock
	p.mtx.Lock()
	state := p.data
	p.mtx.Unlock()
	// create an instance of FsmGroups
	fsmGroups := &partipb.FsmGroups{
		Groups: make(map[string]*partipb.FsmGroup),
	}
	// iterate the state data and build the FsmGroups
	for group, kv := range state {
		// create a data to hold the key/value pair record
		data := make(map[string]*anypb.Any)
		for k, v := range kv {
			// no need to handle the error because v is valid proto message
			record, _ := anypb.New(v)
			// set the appropriate data key/value pair
			data[k] = record
		}
		// add to the fsmGroups
		fsmGroups.Groups[group] = &partipb.FsmGroup{Data: data}
	}
	// let us snapshot the data
	return &snapshot{groups: fsmGroups}, nil
}

// Restore is used to restore an FSM from a snapshot. It is not called
// concurrently with any other command. The FSM must discard all previous
// state before restoring the snapshot.
func (p *ProtoFsm) Restore(snapshot io.ReadCloser) error {
	// let us read the snapshot data
	bytea, err := io.ReadAll(snapshot)
	// handle the error
	if err != nil {
		return err
	}
	// let us unpack the byte array
	fmsGroups := new(partipb.FsmGroups)
	if err := proto.Unmarshal(bytea, fmsGroups); err != nil {
		return errors.Wrap(err, "failed to unmarshal the FSM snapshot")
	}

	// acquire the lock because we are building the fsm state
	p.mtx.Lock()
	// release the lock once we are done building the FSM state
	defer p.mtx.Unlock()
	// iterate the fmsGroups records and build the FSM state
	for group, fsmGroup := range fmsGroups.GetGroups() {
		data := make(map[string]proto.Message)
		for k, v := range fsmGroup.GetData() {
			message, _ := v.UnmarshalNew()
			data[k] = message
		}
		p.data[group] = data
	}
	return snapshot.Close()
}

// Get retrieves a value from the fsm storage
// TODO: make local
func (p *ProtoFsm) Get(group string, key string) (proto.Message, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	if groupMap, ok := p.data[group]; ok {
		if record, ok := groupMap[key]; ok {
			return record, nil
		}
	}
	return nil, nil
}

// put writes a record into the fsm storage
func (p *ProtoFsm) put(group string, key string, data proto.Message) error {
	if group == "" {
		return errors.New("missing group")
	}
	if key == "" {
		return errors.New("missing key")
	}
	if data == nil {
		return errors.New("missing data")
	}
	p.mtx.Lock()
	defer p.mtx.Unlock()
	groupMap, groupExists := p.data[group]
	if !groupExists {
		groupMap = make(map[string]proto.Message)
		p.data[group] = groupMap
	}
	groupMap[key] = data
	return nil
}

// remove a key from storage
func (p *ProtoFsm) remove(group, key string) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	if groupMap, groupExists := p.data[group]; groupExists {
		delete(groupMap, key)
	}
}
