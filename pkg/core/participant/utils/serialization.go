package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
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

// 密文序列化为base64字符串
func SerializeCiphertext(ct *rlwe.Ciphertext) (string, error) {
	data, err := ct.MarshalBinary()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// base64字符串反序列化为密文
func DeserializeCiphertext(s string, params interface{}) (*rlwe.Ciphertext, error) {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}

	// 直接尝试断言为rlwe.Parameters接口
	p, ok := params.(rlwe.Parameters)
	if !ok {
		return nil, fmt.Errorf("参数类型不支持，需要实现rlwe.Parameters接口，实际类型: %T", params)
	}

	ct := rlwe.NewCiphertext(p, 1, p.MaxLevel())
	if err := ct.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	return ct, nil
}
