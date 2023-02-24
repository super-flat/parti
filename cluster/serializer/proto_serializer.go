package serializer

import (
	"errors"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// ProtoSerializer is the default Serializer used in Parti
type ProtoSerializer struct{}

var _ Serializer = &ProtoSerializer{}

// NewProtoSerializer creates an instance of ProtoSerializer
func NewProtoSerializer() *ProtoSerializer {
	return &ProtoSerializer{}
}

// Serialize is used to serialize and data to a []byte
func (serde ProtoSerializer) Serialize(data interface{}) ([]byte, error) {
	if data == nil {
		return nil, errors.New("empty data")
	}
	switch x := data.(type) {
	case proto.Message:
		result, err := serde.SerializeProto(x)
		if err != nil {
			return nil, err
		}
		return result, nil
	default:
		return nil, errors.New("unknown type")
	}
}

// Deserialize is used to deserialize []byte to interface{}
func (serde ProtoSerializer) Deserialize(data []byte) (interface{}, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}
	result, err := serde.DeserializeProto(data)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (serde ProtoSerializer) SerializeProto(data proto.Message) ([]byte, error) {
	if data == nil {
		return nil, errors.New("cannot serialize empty proto data")
	}
	anyMsg, err := anypb.New(data)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(anyMsg)
}

func (serde ProtoSerializer) DeserializeProto(data []byte) (proto.Message, error) {
	if len(data) == 0 {
		return nil, errors.New("cannot deserialize empty bytes into proto")
	}
	var anyMsg anypb.Any
	if err := proto.Unmarshal(data, &anyMsg); err != nil {
		return nil, err
	}
	return anyMsg.UnmarshalNew()
}
