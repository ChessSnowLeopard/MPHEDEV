package test

import (
	"fmt"
	"os"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// TestPublicKey 测试公钥功能
func TestPublicKey(params ckks.Parameters, encoder *ckks.Encoder, encryptor *rlwe.Encryptor,
	decryptorAgg *rlwe.Decryptor) {
	fmt.Println("\n===== 测试公钥功能 =====")

	LogSlots := params.LogMaxSlots()
	Slots := 1 << LogSlots

	// 生成测试数据
	values := generateTestValues(Slots)

	// 编码和加密
	pt := ckks.NewPlaintext(params, params.MaxLevel())
	if err := encoder.Encode(values, pt); err != nil {
		panic(err)
	}

	ct, err := encryptor.EncryptNew(pt)
	if err != nil {
		panic(err)
	}

	// 解密和解码
	dect := decryptorAgg.DecryptNew(ct)
	decoded := make([]complex128, Slots)
	if err := encoder.Decode(dect, decoded); err != nil {
		panic(err)
	}

	fmt.Printf("原始值前3个: %v\n", values[:3])
	fmt.Printf("解密值前3个: %v\n", decoded[:3])
	fmt.Println("公钥测试通过")
}

// TestRelinearizationKey 测试重线性化密钥功能
func TestRelinearizationKey(params ckks.Parameters, evk *rlwe.MemEvaluationKeySet,
	encoder *ckks.Encoder, encryptor *rlwe.Encryptor,
	decryptorAgg *rlwe.Decryptor) {
	fmt.Println("\n===== 测试重线性化密钥功能 =====")

	LogSlots := params.LogMaxSlots()
	Slots := 1 << LogSlots

	// 生成两个测试明文
	values1 := generateTestValues(Slots)
	values2 := generateTestValues(Slots)

	// 编码和加密
	pt1 := ckks.NewPlaintext(params, params.MaxLevel())
	pt2 := ckks.NewPlaintext(params, params.MaxLevel())
	if err := encoder.Encode(values1, pt1); err != nil {
		panic(err)
	}
	if err := encoder.Encode(values2, pt2); err != nil {
		panic(err)
	}

	ct1, err := encryptor.EncryptNew(pt1)
	if err != nil {
		panic(err)
	}
	ct2, err := encryptor.EncryptNew(pt2)
	if err != nil {
		panic(err)
	}

	// 执行乘法并重线性化
	evaluator := ckks.NewEvaluator(params, evk)
	ctMul, err := evaluator.MulRelinNew(ct1, ct2)
	if err != nil {
		panic(err)
	}

	// 解密乘法结果
	dectMul := decryptorAgg.DecryptNew(ctMul)
	decodedMul := make([]complex128, Slots)
	if err = encoder.Decode(dectMul, decodedMul); err != nil {
		panic(err)
	}

	// 验证结果
	fmt.Printf("明文1[0] * 明文2[0] = %v\n", values1[0]*values2[0])
	fmt.Printf("解密乘法结果[0] = %v\n", decodedMul[0])
	fmt.Println("重线性化密钥测试通过")
}

// TestGaloisKeys 测试伽罗瓦密钥功能
func TestGaloisKeys(params ckks.Parameters, N int, evk *rlwe.MemEvaluationKeySet,
	galEls []uint64, skAgg *rlwe.SecretKey) {
	fmt.Println("\n===== 测试伽罗瓦密钥功能 =====")

	// 设定理论上允许的噪声上限
	noise := multiparty.NoiseGaloisKey(params.Parameters, N)
	validKeys := 0

	for _, galEl := range galEls {
		if gk, err := evk.GetGaloisKey(galEl); err != nil {
			fmt.Printf("伽罗瓦元素 %d 对应密钥未找到\n", galEl)
		} else {
			if noise < rlwe.NoiseGaloisKey(gk, skAgg, params.Parameters) {
				fmt.Printf("伽罗瓦元素 %d 对应的密钥噪声过大\n", galEl)
				os.Exit(1)
			}
			validKeys++
		}
	}

	fmt.Printf("成功验证 %d 个伽罗瓦密钥\n", validKeys)
	fmt.Println("伽罗瓦密钥测试通过")
}
