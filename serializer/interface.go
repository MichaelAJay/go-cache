package serializer

// Serializer defines the interface for serializing and deserializing cache values
type Serializer interface {
	// Serialize converts a value to bytes
	Serialize(value interface{}) ([]byte, error)

	// Deserialize converts bytes back to a value
	Deserialize(data []byte, valueType interface{}) error
}
