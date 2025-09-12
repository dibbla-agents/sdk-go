package hash

import (
	"encoding/json"
	"fmt"

	"github.com/spaolacci/murmur3"
)

func Generate(inputs ...interface{}) uint64 {
	hash := murmur3.New64()

	for _, input := range inputs {
		var bytes []byte
		switch v := input.(type) {
		case string:
			bytes = []byte(v)
		case []byte:
			bytes = v
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			bytes = []byte(fmt.Sprintf("%d", v))
		case float32, float64:
			bytes = []byte(fmt.Sprintf("%f", v))
		default:
			if b, err := json.Marshal(v); err == nil {
				bytes = b
			}
		}

		hash.Write(bytes)
	}

	return hash.Sum64()
}
