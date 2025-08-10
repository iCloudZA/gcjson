package iterator

import (
	"github.com/iCloudZA/gcjson/convert"
	"github.com/tidwall/gjson"
)

func EachObject(v any, path string, fn func(k string, r gjson.Result) bool) bool {
	b, err := convert.From(v)
	if err != nil {
		return false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return false
	}
	ok := false
	r.ForEach(func(k, v gjson.Result) bool { ok = true; return fn(k.String(), v) })
	return ok
}

func EachArray(v any, path string, fn func(i int, r gjson.Result) bool) bool {
	b, err := convert.From(v)
	if err != nil {
		return false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return false
	}
	i := 0
	r.ForEach(func(_, v gjson.Result) bool { keep := fn(i, v); i++; return keep })
	return i > 0
}

func EachObjectBytes(v any, path string, fn func(keyBytes []byte, val gjson.Result) bool) bool {
	b, err := convert.From(v)
	if err != nil {
		return false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return false
	}
	hit := false
	r.ForEach(func(k, v gjson.Result) bool {
		if k.Index < 0 || k.Index+len(k.Raw) > len(b) {
			return true
		}
		kb := b[k.Index : k.Index+len(k.Raw)]
		if len(kb) >= 2 && kb[0] == '"' && kb[len(kb)-1] == '"' {
			kb = kb[1 : len(kb)-1]
		}
		hit = true
		return fn(kb, v)
	})
	return hit
}

func EachArrayZero(v any, path string, fn func(i int, r gjson.Result) bool) bool {
	b, err := convert.From(v)
	if err != nil {
		return false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return false
	}
	i := 0
	r.ForEach(func(_, v gjson.Result) bool { keep := fn(i, v); i++; return keep })
	return i > 0
}

func ForEachArrayResult(v any, path string, fn func(idx int, r gjson.Result) bool) bool {
	b, err := convert.From(v)
	if err != nil {
		return false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return false
	}
	i := 0
	r.ForEach(func(_, val gjson.Result) bool {
		ok := fn(i, val)
		i++
		return ok
	})
	return i > 0
}

func ForEachObjectResult(v any, path string, fn func(key string, r gjson.Result) bool) bool {
	b, err := convert.From(v)
	if err != nil {
		return false
	}
	r := gjson.GetBytes(b, path)
	if len(r.Raw) == 0 {
		return false
	}
	count := 0
	r.ForEach(func(k, val gjson.Result) bool {
		count++
		return fn(k.String(), val)
	})
	return count > 0
}
