package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
)

func EncodeShare(share interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(share); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DecodeShare(data []byte, share interface{}) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(share)
}

func EncodeToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func DecodeFromBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
