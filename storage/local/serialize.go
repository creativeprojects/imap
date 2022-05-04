package local

import (
	"bytes"
	"encoding/gob"
	"errors"
	"strconv"
)

func SerializeData(fields map[string]string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := gob.NewEncoder(buffer)
	err := encoder.Encode(fields)
	return buffer.Bytes(), err
}

func DeserializeData(input []byte) (map[string]string, error) {
	output := make(map[string]string)
	decoder := gob.NewDecoder(bytes.NewBuffer(input))
	err := decoder.Decode(&output)
	return output, err
}

func SerializeInt(value int) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := gob.NewEncoder(buffer)
	err := encoder.Encode(value)
	return buffer.Bytes(), err
}

func DeserializeInt(input []byte) (int, error) {
	output := 0
	decoder := gob.NewDecoder(bytes.NewBuffer(input))
	err := decoder.Decode(&output)
	return output, err
}

func SerializeObject[T any](data *T) ([]byte, error) {
	if data == nil {
		return nil, errors.New("cannot serialize nil object")
	}
	buffer := &bytes.Buffer{}
	encoder := gob.NewEncoder(buffer)
	err := encoder.Encode(data)
	return buffer.Bytes(), err
}

func DeserializeObject[T any](input []byte) (*T, error) {
	output := new(T)
	decoder := gob.NewDecoder(bytes.NewBuffer(input))
	err := decoder.Decode(&output)
	return output, err
}

func SerializeUID(prefix string, uid uint64) []byte {
	return []byte(prefix + strconv.FormatUint(uid, 32))
}

func DeserializeUID(prefix string, key []byte) uint64 {
	key = bytes.TrimPrefix(key, []byte(prefix))
	uid, _ := strconv.ParseUint(string(key), 10, 32)
	return uid
}
