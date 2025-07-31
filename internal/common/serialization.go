package common

import (
	"fmt"

	"github.com/MichaelAJay/go-serializer"
)

// GetSerializer returns a serializer instance for the given format
// This provides a centralized way to get serializers across all providers
func GetSerializer(format serializer.Format) (serializer.Serializer, error) {
	switch format {
	case serializer.JSON:
		return serializer.NewJSONSerializer(), nil
	case serializer.Binary:
		return serializer.NewGobSerializer(), nil
	case serializer.Msgpack:
		return serializer.NewMsgpackSerializer(), nil
	default:
		return nil, fmt.Errorf("unsupported serializer format: %s", format)
	}
}

// GetDefaultSerializerFormat returns the default serializer format
func GetDefaultSerializerFormat() serializer.Format {
	return serializer.JSON
}

// EstimateSerializedSize estimates the serialized size of a value
// This is useful for cache size calculations and memory management
func EstimateSerializedSize(value any) int64 {
	if value == nil {
		return 0
	}

	// This is a rough estimation - in a real implementation,
	// you might want to actually serialize and measure
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case int, int32, int64, float32, float64:
		return 8 // rough estimate for numeric types
	case bool:
		return 1
	default:
		// For complex types, use a rough multiplier
		// In practice, you might cache actual serialized sizes
		return 100 // rough estimate
	}
}