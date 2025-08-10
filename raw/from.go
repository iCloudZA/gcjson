package raw

import (
	"encoding/json"
	"errors"

	"github.com/icloudza/gcjson/zeronode"
)

var MaxJSONSize = 10 << 20 // 10MB

func From(v any) (zeronode.Node, error) {
	switch x := v.(type) {
	case nil:
		return zeronode.Node{}, errors.New("nil input")
	case []byte:
		if len(x) > MaxJSONSize {
			return zeronode.Node{}, errors.New("json too large")
		}
		if !quickValidateJSON(x) {
			return zeronode.Node{}, errors.New("invalid json")
		}
		return zeronode.FromBytes(x), nil
	case string:
		if len(x) > MaxJSONSize {
			return zeronode.Node{}, errors.New("json too large")
		}
		if !quickValidateJSON([]byte(x)) {
			return zeronode.Node{}, errors.New("invalid json")
		}
		return zeronode.FromBytes([]byte(x)), nil
	case zeronode.Node:
		return x, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return zeronode.Node{}, err
		}
		if len(b) > MaxJSONSize {
			return zeronode.Node{}, errors.New("json too large")
		}
		return zeronode.FromBytes(b), nil
	}
}

func quickValidateJSON(b []byte) bool {
	i := 0
	for i < len(b) && (b[i] == ' ' || b[i] == '\n' || b[i] == '\r' || b[i] == '\t') {
		i++
	}
	if i >= len(b) {
		return false
	}
	switch b[i] {
	case '{', '[', '"', 't', 'f', 'n', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	default:
		return false
	}
}
