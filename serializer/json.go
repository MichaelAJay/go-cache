package serializer

import (
	"encoding/json"
)

// JSONSerializer implements the Serializer interface using JSON encoding
type JSONSerializer struct{}

// NewJSONSerializer creates a new JSON serializer
func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

// Serialize converts a value to JSON bytes
func (s *JSONSerializer) Serialize(value any) ([]byte, error) {
	return json.Marshal(value)
}

// Deserialize converts JSON bytes back to a value
func (s *JSONSerializer) Deserialize(data []byte, valueType any) error {
	return json.Unmarshal(data, valueType)
}
