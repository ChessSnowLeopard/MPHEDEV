// 序列化与编码工具函数
// 提供结构体与字节流、Base64字符串之间的转换，便于网络传输和存储
package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
)

// EncodeShare 将结构体（如密钥份额、CRP等）序列化为字节流
// 参数：share 需要序列化的结构体
// 返回：序列化后的字节切片和错误信息
func EncodeShare(share interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	//创建缓冲区，gob编码器指向该缓冲区
	if err := enc.Encode(share); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodeShare 将字节流反序列化为结构体
// 参数：data 字节流，share 反序列化目标（指针）
// 返回：错误信息
func DecodeShare(data []byte, share interface{}) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(share)
}

// EncodeToBase64 将字节流编码为Base64字符串，便于网络传输
// 参数：data 字节流
// 返回：Base64字符串
func EncodeToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeFromBase64 将Base64字符串解码为字节流
// 参数：s Base64字符串
// 返回：字节流和错误信息
func DecodeFromBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
