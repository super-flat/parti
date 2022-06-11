package fsm

import (
	"errors"
	"io"
	"log"
	"sync"

	"github.com/hashicorp/raft"
	"github.com/super-flat/parti/cluster/raftwrapper/serializer"
	partipb "github.com/super-flat/parti/partipb/parti/v1"
	"google.golang.org/protobuf/proto"
)

type ProtoFsm struct {
	data map[string]map[string]proto.Message
	mtx  *sync.Mutex
	ser  *serializer.ProtoSerializer
}

var _ raft.FSM = &ProtoFsm{}

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
func (p *ProtoFsm) Apply(raftLog *raft.Log) interface{} {
	if raftLog == nil || raftLog.Data == nil {
		log.Println("Raft received nil log apply!")
		return nil
	}
	switch raftLog.Type {
	case raft.LogCommand:
		msg, err := p.ser.DeserializeProto(raftLog.Data)
		if err != nil {
			return err
		}
		// TODO: copied this from easyraft impl, but dont love it.
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
func (p *ProtoFsm) Snapshot() (raft.FSMSnapshot, error) {
	return nil, errors.New("not implemented")
}

// Restore is used to restore an FSM from a snapshot. It is not called
// concurrently with any other command. The FSM must discard all previous
// state before restoring the snapshot.
func (p *ProtoFsm) Restore(snapshot io.ReadCloser) error {
	return errors.New("not implemented")
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
