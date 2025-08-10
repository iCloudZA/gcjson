package raw

import (
	"github.com/icloudza/gcjson/convert"
	"github.com/tidwall/gjson"
)

func Get(v any, path string) (string, bool) {
	b, err := convert.From(v)
	if err != nil {
		return "", false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return "", false
	}
	return r.Raw, true
}

func GetBytes(v any, path string) ([]byte, bool) {
	b, err := convert.From(v)
	if err != nil {
		return nil, false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return nil, false
	}
	if r.Index >= 0 && r.Index+len(r.Raw) <= len(b) {
		return b[r.Index : r.Index+len(r.Raw)], true
	}
	return []byte(r.Raw), true
}

func GetMany(v any, paths ...string) ([]string, error) {
	b, err := convert.From(v)
	if err != nil {
		return nil, err
	}
	results := gjson.GetManyBytes(b, paths...)
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Raw
	}
	return out, nil
}

func GetManyBytes(v any, paths ...string) ([][]byte, error) {
	b, err := convert.From(v)
	if err != nil {
		return nil, err
	}
	rs := gjson.GetManyBytes(b, paths...)
	out := make([][]byte, len(rs))
	for i, r := range rs {
		if len(r.Raw) == 0 {
			continue
		}
		if r.Index >= 0 && r.Index+len(r.Raw) <= len(b) {
			out[i] = b[r.Index : r.Index+len(r.Raw)]
		} else {
			out[i] = []byte(r.Raw)
		}
	}
	return out, nil
}
