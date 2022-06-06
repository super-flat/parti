package raftwrapper

import (
	"errors"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type ProtoAnySerializer struct{}

func NewProtoAnySerializer() *ProtoAnySerializer {
	return &ProtoAnySerializer{}
}

func (p ProtoAnySerializer) Serialize(data proto.Message) ([]byte, error) {
	if data == nil {
		return nil, errors.New("cannot serialize empty proto data")
	}
	anyMsg, err := anypb.New(data)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(anyMsg)
}

func (p ProtoAnySerializer) Deserialize(data []byte) (proto.Message, error) {
	if data == nil || len(data) == 0 {
		return nil, errors.New("cannot deserialize empty bytes into proto")
	}
	var anyMsg anypb.Any
	if err := proto.Unmarshal(data, &anyMsg); err != nil {
		return nil, err
	}
	return anyMsg.UnmarshalNew()
}

type LegacySerializer struct {
	ser *ProtoAnySerializer
}

func NewLegacySerializer() *LegacySerializer {
	return &LegacySerializer{
		ser: &ProtoAnySerializer{},
	}
}

// Serialize is used to serialize and data to a []byte
func (l LegacySerializer) Serialize(data interface{}) ([]byte, error) {
	if data == nil {
		return nil, errors.New("empty data")
	}
	switch x := data.(type) {
	case proto.Message:
		result, err := l.ser.Serialize(x)
		if err != nil {
			return nil, err
		}
		return result, nil
	default:
		return nil, errors.New("unknown type")
	}
}

// Deserialize is used to deserialize []byte to interface{}
func (l LegacySerializer) Deserialize(data []byte) (interface{}, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}
	result, err := l.ser.Deserialize(data)
	if err != nil {
		return nil, err
	}
	return result, nil
}
