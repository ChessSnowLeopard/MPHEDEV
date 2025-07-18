package main

import (
	"MNIST-CNN/pkg/homomorphic"
	"fmt"
	"log"
	"math"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

func main() {
	fmt.Println("=== Softmax功能测试 ===")

	// 初始化CKKS参数
	params, err := ckks.NewParametersFromLiteral(
		ckks.ParametersLiteral{
			LogN:            14,
			LogQ:            []int{55, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45},
			LogP:            []int{61},
			LogDefaultScale: 45,
		})
	if err != nil {
		log.Fatal(err)
	}

	// 生成密钥
	kgen := rlwe.NewKeyGenerator(params)
	sk := kgen.GenSecretKeyNew()
	rlk := kgen.GenRelinearizationKeyNew(sk)
	evk := rlwe.NewMemEvaluationKeySet(rlk)

	// 创建编码器、加密器、解密器和评估器
	encoder := ckks.NewEncoder(params)
	encryptor := rlwe.NewEncryptor(params, sk)
	decryptor := rlwe.NewDecryptor(params, sk)
	evaluator := ckks.NewEvaluator(params, evk)

	slots := 1 << params.LogMaxSlots()

	// 模拟神经网络输出（10个类别，4个样本）
	numClasses := 10
	numSamples := 4

	// 创建测试数据（修改为更真实的范围）
	fmt.Println("\n创建测试数据...")
	outputs := make([][]float64, numClasses)
	for i := 0; i < numClasses; i++ {
		outputs[i] = make([]float64, numSamples)
		for j := 0; j < numSamples; j++ {
			// 生成更真实的测试值（在-3到3之间）
			if i == j%numClasses {
				outputs[i][j] = 1.5 + float64(i)*0.05 // 正确类别的较大值
			} else {
				outputs[i][j] = -1.0 + float64(i)*0.02 - float64(j)*0.1 // 其他类别的较小值
			}
		}
	}

	// 打印原始输出
	fmt.Println("\n原始神经网络输出：")
	for j := 0; j < numSamples; j++ {
		fmt.Printf("样本%d: ", j)
		for i := 0; i < numClasses; i++ {
			fmt.Printf("%6.3f ", outputs[i][j])
		}
		fmt.Println()
	}

	// 加密输出
	fmt.Println("\n加密数据...")
	encryptedOutputs := make([]*rlwe.Ciphertext, numClasses)
	for i := 0; i < numClasses; i++ {
		// 准备复数向量
		values := make([]complex128, slots)
		for j := 0; j < numSamples && j < slots; j++ {
			values[j] = complex(outputs[i][j], 0)
		}

		// 编码和加密
		pt := ckks.NewPlaintext(params, params.MaxLevel())
		encoder.Encode(values, pt)
		encryptedOutputs[i], _ = encryptor.EncryptNew(pt)
	}

	// 计算同态softmax
	fmt.Println("\n计算同态Softmax...")
	softmaxOutputs, err := homomorphic.HomomorphicSoftmax(encryptedOutputs, params, evaluator, encoder, slots)
	if err != nil {
		log.Fatalf("Softmax计算失败: %v", err)
	}

	// 验证结果
	homomorphic.VerifySoftmax(encryptedOutputs, softmaxOutputs, decryptor, encoder, slots, numSamples)

	// 计算明文softmax对比
	fmt.Println("\n明文Softmax计算：")
	for j := 0; j < numSamples; j++ {
		// 计算exp和sum
		expSum := 0.0
		expValues := make([]float64, numClasses)
		for i := 0; i < numClasses; i++ {
			expValues[i] = math.Exp(outputs[i][j])
			expSum += expValues[i]
		}

		// 计算softmax
		fmt.Printf("样本%d Softmax: ", j)
		maxIdx := 0
		maxProb := 0.0
		for i := 0; i < numClasses; i++ {
			prob := expValues[i] / expSum
			if prob > maxProb {
				maxProb = prob
				maxIdx = i
			}
			fmt.Printf("%6.4f ", prob)
		}
		fmt.Printf(" -> 预测类别: %d\n", maxIdx)
	}

	fmt.Println("\n=== 测试完成 ===")
}
