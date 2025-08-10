package convert

import (
	"errors"
	"unicode/utf8"
	"unsafe"

	"github.com/bytedance/sonic"
)

const MaxJSONSize = 10 << 20 // 10MB

//go:nosplit
func UnsafeStringToBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

//go:nosplit
func ValidUTF8(b []byte) bool {
	return utf8.Valid(b)
}

func From(v any) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return nil, errors.New("nil input")
	case string:
		b := UnsafeStringToBytes(x)
		if !ValidUTF8(b) {
			return nil, errors.New("invalid UTF-8 string")
		}
		return b, nil
	case []byte:
		if !ValidUTF8(x) {
			return nil, errors.New("invalid UTF-8 bytes")
		}
		return x, nil
	case *string:
		if x == nil {
			return nil, errors.New("nil string pointer")
		}
		b := UnsafeStringToBytes(*x)
		if !ValidUTF8(b) {
			return nil, errors.New("invalid UTF-8 string")
		}
		return b, nil
	default:
		return sonic.ConfigStd.Marshal(v)
	}
}
