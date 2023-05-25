package main

import (
	"encoding/json"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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
	return protojson.Unmarshal(j, pb)
}

func structToJSON(pb proto.Message) ([]byte, error) {
	b, err := (&protojson.MarshalOptions{UseProtoNames: true}).Marshal(pb)
	if err != nil {
		return nil, err
	}
	return b, nil
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
