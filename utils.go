package main

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"sigs.k8s.io/yaml"
)

// Convert a YAML list into a slice of interfaces
func unmarshalYAMLSlice(b []byte) ([]interface{}, error) {
	var raw []interface{}
	err := yaml.Unmarshal(b, &raw)
	return raw, err
}

// To convert into a valid proto.Message, it needs to first
// be encoded into JSON, then loaded through jsonpb
func convertToPb(data interface{}, pb proto.Message) error {
	j, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return jsonpb.Unmarshal(bytes.NewReader(j), pb)
}

func structToJSON(pb proto.Message) ([]byte, error) {
	var b bytes.Buffer
	if err := (&jsonpb.Marshaler{OrigName: true}).Marshal(&b, pb); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func structToYAML(pb proto.Message) ([]byte, error) {
	j, err := structToJSON(pb)
	if err != nil {
		return nil, err
	}
	return yaml.JSONToYAML(j)
}

func k8sSplitName(name string) (string, string) {
	s := strings.SplitN(name, "/", 2)
	return s[0], s[1]
}
