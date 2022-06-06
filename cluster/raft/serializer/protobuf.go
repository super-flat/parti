package serializer

import (
	"errors"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type ProtoAnySerializer struct{}

func (p ProtoAnySerializer) Serialize(data proto.Message) ([]byte, error) {
	anyMsg, err := anypb.New(data)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(anyMsg)
}

func (p ProtoAnySerializer) Deserialize(data []byte) (proto.Message, error) {
	var anyMsg *anypb.Any
	if err := proto.Unmarshal(data, anyMsg); err != nil {
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
	switch x := data.(type) {
	case proto.Message:
		return l.ser.Serialize(x)
	default:
		return nil, errors.New("unknown type")
	}
}

// Deserialize is used to deserialize []byte to interface{}
func (l LegacySerializer) Deserialize(data []byte) (interface{}, error) {
	return l.ser.Deserialize(data)
}
