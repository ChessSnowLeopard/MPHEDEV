package participant

import (
	"gonum.org/v1/gonum/mat"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// User从网络获取公钥，将数据加密后发送到网络
type User struct {
	Params    ckks.Parameters
	pk        *rlwe.PublicKey
	Encoder   *ckks.Encoder
	Encryptor *rlwe.Encryptor
	Decryptor *rlwe.Decryptor // 仅用于校验
}

func NewUser(params ckks.Parameters, pk *rlwe.PublicKey, sk *rlwe.SecretKey) *User {
	encoder := ckks.NewEncoder(params)
	encryptor := ckks.NewEncryptor(params, pk)

	var decryptor *rlwe.Decryptor
	if sk != nil {
		// 仅用于检验结果
		decryptor = ckks.NewDecryptor(params, sk)
	}

	return &User{
		Params:    params,
		pk:        pk,
		Encoder:   encoder,
		Encryptor: encryptor,
		Decryptor: decryptor,
	}
}

// EncryptVector encrypts a vector for processing by the neural network
func (u *User) EncryptVector(input *mat.VecDense) (*rlwe.Ciphertext, error) {
	// Convert the input vector to a slice of complex values
	values := make([]complex128, input.Len())
	for i := 0; i < input.Len(); i++ {
		values[i] = complex(input.AtVec(i), 0)
	}
	// 编码和加密
	pt := ckks.NewPlaintext(u.Params, u.Params.MaxLevel())
	if err := u.Encoder.Encode(values, pt); err != nil {
		panic(err)
	}

	ct, err := u.Encryptor.EncryptNew(pt)
	if err != nil {
		panic(err)
	}

	return ct, nil
}

// 解密(仅用于测试)
func (u *User) DecryptVector(ct *rlwe.Ciphertext) (*mat.VecDense, error) {
	if u.Decryptor == nil {
		return nil, nil // No secret key available for decryption
	}
	LogSlots := u.Params.LogMaxSlots()
	Slots := 1 << LogSlots
	// 解密和解码
	dect := u.Decryptor.DecryptNew(ct)
	decoded := make([]complex128, Slots)
	if err := u.Encoder.Decode(dect, decoded); err != nil {
		panic(err)
	}

	// 将解码后的值转换为实数向量
	result := mat.NewVecDense(len(decoded), nil)
	for i := 0; i < len(decoded); i++ {
		result.SetVec(i, real(decoded[i]))
	}

	return result, nil
}
