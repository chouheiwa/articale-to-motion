package castbeats

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// field 是 JSON 对象里的一个键值对，value 保留源文件里的原始字节。
type field struct {
	key   string
	value json.RawMessage
}

// decodeObject 把一个 JSON 对象拆成保序的键值对，每个值都是原始字节。
//
// 不用 map[string]json.RawMessage 是因为 map 会丢掉键的顺序：本命令只改
// cast.beats 一个字段，重排其它键会让每次运行都产生一大片与本命令无关的
// diff，作者再也看不出来 am 到底改了什么。
func decodeObject(raw []byte) ([]field, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("期望一个 JSON 对象")
	}
	var fields []field
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("JSON 对象的键不是字符串")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		fields = append(fields, field{key: key, value: value})
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return fields, nil
}

// encodeObject 把保序键值对拼回紧凑 JSON 对象。值原样透出，只做 Compact，
// 所以数字的写法（0.78 不会变成 0.7800000000000001）与字符串的转义都不变。
func encodeObject(fields []field) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte('{')
	for i, f := range fields {
		if i > 0 {
			buffer.WriteByte(',')
		}
		key, err := json.Marshal(f.key)
		if err != nil {
			return nil, err
		}
		buffer.Write(key)
		buffer.WriteByte(':')
		if err := json.Compact(&buffer, f.value); err != nil {
			return nil, err
		}
	}
	buffer.WriteByte('}')
	return buffer.Bytes(), nil
}

// setField 覆盖已有键的值，键不存在时追加到末尾。返回新的切片。
func setField(fields []field, key string, value json.RawMessage) []field {
	for i := range fields {
		if fields[i].key == key {
			fields[i].value = value
			return fields
		}
	}
	return append(fields, field{key: key, value: value})
}

// findField 取某个键的原始值，不存在时第二个返回值为 false。
func findField(fields []field, key string) (json.RawMessage, bool) {
	for _, f := range fields {
		if f.key == key {
			return f.value, true
		}
	}
	return nil, false
}
