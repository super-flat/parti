package fsm

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"reflect"
	"strings"
	"sync"

	"github.com/hashicorp/raft"
	"github.com/super-flat/parti/cluster/raft/serializer"
)

type BaseFSMSnapshot struct {
	sync.Mutex
	fsm *RoutingFSM
}

func NewBaseFSMSnapshot(fsm *RoutingFSM) raft.FSMSnapshot {
	return &BaseFSMSnapshot{fsm: fsm}
}

func (r *BaseFSMSnapshot) Persist(sink raft.SnapshotSink) error {
	r.Lock()
	snapshotData, err := r.fsm.ser.Serialize(r.fsm.services)
	if err != nil {
		return err
	}
	_, err = sink.Write(snapshotData)
	return err
}

func (r *BaseFSMSnapshot) Release() {
	r.Unlock()
}

type RoutingFSM struct {
	services            map[string]Service
	ser                 serializer.Serializer
	reqDataTypes        []interface{}
	reqServiceDataTypes map[string]Service
}

func NewRoutingFSM(services []Service) FSM {
	servicesMap := map[string]Service{}
	for _, service := range services {
		servicesMap[service.Name()] = service
	}
	return &RoutingFSM{
		services:            servicesMap,
		reqDataTypes:        []interface{}{},
		reqServiceDataTypes: map[string]Service{},
	}
}

func (r *RoutingFSM) Init(ser serializer.Serializer) {
	r.ser = ser
	for _, service := range r.services {
		r.reqDataTypes = append(r.reqDataTypes, service.GetReqDataTypes()...)
		for _, dt := range service.GetReqDataTypes() {
			r.reqServiceDataTypes[fmt.Sprintf("%#v", dt)] = service
		}
	}
}

func (r *RoutingFSM) Apply(log *raft.Log) interface{} {
	switch log.Type {
	case raft.LogCommand:
		// deserialize
		payload, err := r.ser.Deserialize(log.Data)
		if err != nil {
			return err
		}
		payloadMap := payload.(map[string]interface{})

		// check request type
		var fields []string
		for k, _ := range payloadMap {
			fields = append(fields, k)
		}
		// routing request to service
		foundType, err := getTargetType(r.reqDataTypes, fields)
		if err == nil {
			for typeName, service := range r.reqServiceDataTypes {
				if strings.EqualFold(fmt.Sprintf("%#v", foundType), typeName) {
					return service.NewLog(foundType, payloadMap)
				}
			}
		}
		return errors.New("unknown request data type")
	}

	return nil
}

func (r *RoutingFSM) Snapshot() (raft.FSMSnapshot, error) {
	return NewBaseFSMSnapshot(r), nil
}

func (r *RoutingFSM) Restore(closer io.ReadCloser) error {
	snapData, err := ioutil.ReadAll(closer)
	if err != nil {
		return err
	}
	servicesData, err := r.ser.Deserialize(snapData)
	if err != nil {
		return err
	}
	s := servicesData.(map[string]interface{})
	for key, service := range r.services {
		err = service.ApplySnapshot(s[key])
		if err != nil {
			log.Printf("Failed to apply snapshot to %q service!\n", key)
		}
	}
	return nil
}

func getTargetType(types []interface{}, expectedFields []string) (interface{}, error) {
	var result interface{} = nil
	for _, i := range types {
		// get current type fields list
		v := reflect.ValueOf(i)
		typeOfS := v.Type()
		currentTypeFields := make([]string, v.NumField())
		for i := 0; i < v.NumField(); i++ {
			currentTypeFields[i] = typeOfS.Field(i).Name
		}

		// compare current vs expected fields
		foundAllFields := true
		for _, expectedField := range expectedFields {
			foundCurrentField := false
			for _, currentTypeField := range currentTypeFields {
				if currentTypeField == expectedField {
					foundCurrentField = true
				}
			}
			if !foundCurrentField {
				foundAllFields = false
			}
		}

		if foundAllFields && len(expectedFields) == len(currentTypeFields) {
			result = i
			break
		}
	}

	if result == nil {
		return nil, errors.New("unknown type!")
	}

	return result, nil
}
