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
	fmt.Println("=== 简单Softmax测试（模拟神经网络输出层）===")

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

	// 模拟一个批次的神经网络输出（10个类别，128个样本）
	numClasses := 10
	numSamples := 128

	fmt.Printf("\n生成测试数据（%d个类别，%d个样本）...\n", numClasses, numSamples)

	// 创建模拟的神经网络输出
	outputs := make([][]float64, numClasses)
	for i := 0; i < numClasses; i++ {
		outputs[i] = make([]float64, numSamples)
		for j := 0; j < numSamples; j++ {
			// 每个样本有一个"正确"的类别
			correctClass := j % numClasses
			if i == correctClass {
				// 正确类别得分较高
				outputs[i][j] = 1.0 + float64(i)*0.01
			} else {
				// 其他类别得分较低
				outputs[i][j] = -0.5 - math.Abs(float64(i-correctClass))*0.1
			}
		}
	}

	// 打印前5个样本的原始输出
	fmt.Println("\n前5个样本的原始输出：")
	for j := 0; j < 5; j++ {
		fmt.Printf("样本%d (真实类别=%d): ", j, j%numClasses)
		for i := 0; i < numClasses; i++ {
			fmt.Printf("%6.3f ", outputs[i][j])
		}
		fmt.Println()
	}

	// 加密输出
	fmt.Println("\n加密神经网络输出...")
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

		fmt.Printf("类别%d密文层级: %d\n", i, encryptedOutputs[i].Level())
	}

	// 计算同态softmax
	fmt.Println("\n执行同态Softmax计算...")
	softmaxOutputs, err := homomorphic.HomomorphicSoftmax(encryptedOutputs, params, evaluator, encoder, slots)
	if err != nil {
		log.Fatalf("Softmax计算失败: %v", err)
	}

	// 验证结果
	fmt.Println("\n验证Softmax结果...")
	homomorphic.VerifySoftmax(encryptedOutputs, softmaxOutputs, decryptor, encoder, slots, numSamples)

	// 检查预测准确率
	fmt.Println("\n=== 预测准确率分析 ===")
	checkPredictionAccuracy(softmaxOutputs, numSamples, decryptor, encoder, slots)

	fmt.Println("\n=== 测试完成 ===")
}

func checkPredictionAccuracy(softmaxOutputs []*rlwe.Ciphertext, numSamples int,
	decryptor *rlwe.Decryptor, encoder *ckks.Encoder, slots int) {

	// 解密并找到每个样本的预测类别
	predictions := make([]int, numSamples)
	confidences := make([]float64, numSamples)

	for sampleIdx := 0; sampleIdx < numSamples; sampleIdx++ {
		maxProb := -1.0
		maxClass := -1

		// 找到概率最大的类别
		for classIdx, ct := range softmaxOutputs {
			pt := decryptor.DecryptNew(ct)
			decoded := make([]complex128, slots)
			encoder.Decode(pt, decoded)

			prob := real(decoded[sampleIdx])
			if prob > maxProb {
				maxProb = prob
				maxClass = classIdx
			}
		}

		predictions[sampleIdx] = maxClass
		confidences[sampleIdx] = maxProb
	}

	// 计算准确率
	correct := 0
	fmt.Println("\n前20个样本的预测结果：")
	fmt.Println("样本 | 真实类别 | 预测类别 | 置信度 | 正确")
	fmt.Println("-----|----------|----------|--------|------")

	for i := 0; i < min(20, numSamples); i++ {
		trueLabel := i % 10
		isCorrect := predictions[i] == trueLabel
		if isCorrect {
			correct++
		}

		fmt.Printf("%4d |    %4d   |    %4d   | %5.2f%% | %s\n",
			i, trueLabel, predictions[i], confidences[i]*100,
			func() string {
				if isCorrect {
					return "✓"
				}
				return "✗"
			}())
	}

	// 计算全部样本
	for i := 20; i < numSamples; i++ {
		if predictions[i] == i%10 {
			correct++
		}
	}

	accuracy := float64(correct) / float64(numSamples) * 100
	fmt.Printf("\n总准确率: %d/%d = %.2f%%\n", correct, numSamples, accuracy)

	if accuracy > 95 {
		fmt.Println("✓ Softmax分类效果良好！")
	} else {
		fmt.Println("⚠ 准确率偏低，可能需要检查计算")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
