package test

import (
	"MPHEDev/pkg/participant"
	"MPHEDev/pkg/protocols"
	"fmt"
	"math"
	_ "math/rand"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// TestMultiPartyDecryption 测试多方解密功能
func TestMultiPartyDecryption(params ckks.Parameters, N int, parties []*participant.Party,
	cloud *participant.Cloud, encoder *ckks.Encoder,
	encryptor *rlwe.Encryptor, decryptorAgg *rlwe.Decryptor) {
	fmt.Println("\n===== 测试多方解密功能 =====")

	LogSlots := params.LogMaxSlots()
	Slots := 1 << LogSlots

	// 初始化待解密的密文
	ciphertexts := make(map[uint64]*rlwe.Ciphertext)
	plaintexts := make(map[uint64][]complex128)

	// 生成测试数据
	for key := uint64(0); key < 3; key++ {
		values := generateTestValues(Slots)

		pt := ckks.NewPlaintext(params, params.MaxLevel())
		if err := encoder.Encode(values, pt); err != nil {
			panic(err)
		}

		ct, err := encryptor.EncryptNew(pt)
		if err != nil {
			panic(err)
		}

		ciphertexts[key] = ct
		plaintexts[key] = values

		fmt.Printf("生成测试密文 %d 完成\n", key)
	}

	// 执行多方解密
	resultPts := protocols.MultiPartyDecryption(params, N, parties, cloud, ciphertexts)

	// 验证结果
	for key, pt := range resultPts {
		decoded := make([]complex128, Slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			panic(err)
		}

		// 计算误差
		var sumErr float64
		for i := 0; i < Slots; i++ {
			diff := plaintexts[key][i] - decoded[i]
			sumErr += complexAbs(diff) * complexAbs(diff)
		}
		avgErr := math.Sqrt(sumErr / float64(Slots))

		fmt.Printf("密文 %d 解密平均误差: %.10f\n", key, avgErr)
	}

	fmt.Println("多方解密测试通过")
}

// TestRefreshOperation 测试刷新操作
func TestRefreshOperation(params ckks.Parameters, encoder *ckks.Encoder,
	encryptor *rlwe.Encryptor, evaluator *ckks.Evaluator) (map[uint64]*rlwe.Ciphertext, map[uint64][]complex128) {
	fmt.Println("\n===== 测试刷新操作准备 =====")

	LogSlots := params.LogMaxSlots()
	Slots := 1 << LogSlots

	// 初始化密文和明文
	ciphertexts := make(map[uint64]*rlwe.Ciphertext)
	plaintextsMul := make(map[uint64][]complex128)

	for key := uint64(0); key < 3; key++ {
		values1 := generateTestValues(Slots)
		values2 := generateTestValues(Slots)

		pt1 := ckks.NewPlaintext(params, params.MaxLevel())
		pt2 := ckks.NewPlaintext(params, params.MaxLevel())
		_ = encoder.Encode(values1, pt1)
		_ = encoder.Encode(values2, pt2)

		ct1, _ := encryptor.EncryptNew(pt1)
		ct2, _ := encryptor.EncryptNew(pt2)

		// 执行乘法
		ctMul, err := evaluator.MulRelinNew(ct1, ct2)
		if err != nil {
			panic(fmt.Sprintf("密文乘法失败, key=%d: %v", key, err))
		}

		// 重新缩放
		ctRescaled := ckks.NewCiphertext(params, ctMul.Degree(), ctMul.Level())
		evaluator.Rescale(ctMul, ctRescaled)

		fmt.Printf("密文 %d 乘法后 Level = %d -> Rescale后 Level = %d\n",
			key, ctMul.Level(), ctRescaled.Level())

		// 保存
		ciphertexts[key] = ctRescaled
		plaintextsMul[key] = make([]complex128, Slots)
		for i := 0; i < Slots; i++ {
			plaintextsMul[key][i] = values1[i] * values2[i]
		}
	}

	return ciphertexts, plaintextsMul
}
