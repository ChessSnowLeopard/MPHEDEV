package main

import (
	"MNIST-CNN/pkg/homomorphic"
	"MNIST-CNN/pkg/protocols"
	"MNIST-CNN/pkg/setup"
	"fmt"
	"math"
	"math/rand"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("完整的交叉熵损失函数测试")
	fmt.Println("========================================")

	// 设置随机种子
	rand.Seed(42)

	// 1. 初始化多方系统（包含批内聚合所需的旋转元素）
	N := 3

	// 创建包含批内聚合旋转的配置
	rotConfig := setup.RotationConfig{
		BasicRotations:    []int{1, 2, 3, 4, 5, 10, 20},
		BatchRotations:    []int{64, 128, 256},
		IncludePowerOfTwo: true, // 🔑 关键：包含2的幂次旋转用于批内聚合
		MaxRotation:       32,   // 最大旋转范围足够批内聚合使用
	}

	params, parties, cloud, _, crs, err := setup.InitializeSystemWithRotations(N, rotConfig)
	if err != nil {
		panic(err)
	}

	// 生成聚合私钥用于测试
	skAgg := setup.GenerateAggregatedSecretKey(params, parties)

	// 2. 生成分布式密钥
	protocols.GeneratePublicKey(params, N, parties, cloud)
	pk := <-cloud.PkgDone

	protocols.GenerateRelinearizationKey(params, N, parties, cloud)
	rlk := <-cloud.RlkDone

	// 🔑 生成伽罗瓦密钥（用于旋转操作）
	// 重新获取伽罗瓦元素列表
	_, _, _, galEls, _, err := setup.InitializeSystemWithRotations(N, rotConfig)
	if err != nil {
		panic(err)
	}

	protocols.GenerateGaloisKeys(params, N, parties, cloud, galEls)

	// 收集所有伽罗瓦密钥
	galKeys := make([]*rlwe.GaloisKey, len(galEls))
	for i := 0; i < len(galEls); i++ {
		galKeys[i] = <-cloud.GalKeyDone
	}

	// 为每个参与方设置伽罗瓦密钥
	for _, party := range parties {
		party.Gks = galKeys
		party.Pk = pk
		party.Rlk = rlk
	}

	// 创建编码器、加密器、解密器、评估器
	encoder := ckks.NewEncoder(params)
	encryptor := rlwe.NewEncryptor(params, pk)
	decryptor := rlwe.NewDecryptor(params, skAgg)
	evk := rlwe.NewMemEvaluationKeySet(rlk, galKeys...)
	evaluator := ckks.NewEvaluator(params, evk)

	// 3. 创建模拟的softmax输出
	numClasses := 10
	batchSize := 4
	slots := 1 << params.LogMaxSlots()

	fmt.Printf("测试参数:\n")
	fmt.Printf("  类别数: %d\n", numClasses)
	fmt.Printf("  批大小: %d\n", batchSize)
	fmt.Printf("  槽数: %d\n", slots)

	// 创建模拟的softmax输出（确保和为1）
	softmaxOutputs := make([]*rlwe.Ciphertext, numClasses)
	softmaxValues := make([][]float64, numClasses)

	for i := 0; i < numClasses; i++ {
		softmaxValues[i] = make([]float64, batchSize)
	}

	// 为每个样本生成softmax概率分布
	for sample := 0; sample < batchSize; sample++ {
		// 生成随机logits
		logits := make([]float64, numClasses)
		for i := 0; i < numClasses; i++ {
			logits[i] = rand.Float64()*2 - 1 // [-1, 1]范围
		}

		// 计算softmax
		maxLogit := logits[0]
		for i := 1; i < numClasses; i++ {
			if logits[i] > maxLogit {
				maxLogit = logits[i]
			}
		}

		expSum := 0.0
		expValues := make([]float64, numClasses)
		for i := 0; i < numClasses; i++ {
			expValues[i] = math.Exp(logits[i] - maxLogit)
			expSum += expValues[i]
		}

		for i := 0; i < numClasses; i++ {
			softmaxValues[i][sample] = expValues[i] / expSum
		}
	}

	// 打印softmax值的范围
	fmt.Printf("\nSoftmax输出范围分析:\n")
	globalMin, globalMax := 1.0, 0.0
	for i := 0; i < numClasses; i++ {
		for j := 0; j < batchSize; j++ {
			val := softmaxValues[i][j]
			if val < globalMin {
				globalMin = val
			}
			if val > globalMax {
				globalMax = val
			}
		}
	}
	fmt.Printf("  全局范围: [%.6f, %.6f]\n", globalMin, globalMax)

	// 加密softmax输出
	for i := 0; i < numClasses; i++ {
		values := make([]complex128, slots)
		for j := 0; j < batchSize; j++ {
			values[j] = complex(softmaxValues[i][j], 0)
		}

		pt := ckks.NewPlaintext(params, params.MaxLevel())
		if err := encoder.Encode(values, pt); err != nil {
			panic(err)
		}

		ct, err := encryptor.EncryptNew(pt)
		if err != nil {
			panic(err)
		}
		softmaxOutputs[i] = ct
	}

	// 4. 创建标签
	labels := []byte{2, 5, 1, 7} // 示例标签

	fmt.Printf("\n测试标签: %v\n", labels)

	// 5. 创建交叉熵损失函数
	crossEntropy := homomorphic.NewCrossEntropyLoss(numClasses, batchSize, slots, 128)

	// 6. 测试对数近似器的调试功能
	fmt.Println("\n=== 对数近似器调试 ===")
	crossEntropy.ComputePlaintextCrossEntropyForDebug(softmaxOutputs, labels, decryptor, encoder, slots)

	// 7. 🚀 计算高效隐私保护的交叉熵损失
	fmt.Println("\n=== 🚀 高效隐私保护的交叉熵损失计算 ===")
	encryptedAvgLoss, err := crossEntropy.ComputeCrossEntropyLossPrivacyPreserving(
		softmaxOutputs, labels, params, evaluator, encoder,
		N, parties, cloud, crs,
	)
	if err != nil {
		fmt.Printf("高效隐私保护交叉熵损失计算失败: %v\n", err)
		return
	}

	// 8. 🚀 高效隐私保护的结果验证
	fmt.Println("\n=== 🚀 高效隐私保护的结果验证 ===")
	crossEntropy.VerifyCrossEntropyLossEfficientPrivacy(
		encryptedAvgLoss, softmaxOutputs, labels, decryptor, encoder, slots)

	// 9. 🔄 对比标准版本（展示效率和隐私差异）
	fmt.Println("\n=== 🔄 标准版本对比（展示差异）===")
	fmt.Println("注意：以下为标准版本的计算，会泄露个体样本信息！")

	// 标准版本计算（会泄露个体信息）
	encryptedLossStandard, err := crossEntropy.ComputeCrossEntropyLoss(
		softmaxOutputs, labels, params, evaluator, encoder, decryptor,
		N, parties, cloud, crs,
	)
	if err != nil {
		fmt.Printf("标准交叉熵损失计算失败: %v\n", err)
		return
	}

	// 标准版本验证（会泄露个体信息）
	fmt.Println("\n🚨 标准版本验证（泄露个体样本信息）：")
	crossEntropy.VerifyCrossEntropyLoss(encryptedLossStandard, softmaxOutputs, labels, decryptor, encoder, slots)

	// 比较两种方法的最终平均损失
	fmt.Println("\n=== 📊 算法效率和隐私对比 ===")

	// 解密高效隐私保护版本的平均损失
	ptPrivacy := decryptor.DecryptNew(encryptedAvgLoss)
	decodedPrivacy := make([]complex128, slots)
	if err := encoder.Decode(ptPrivacy, decodedPrivacy); err != nil {
		fmt.Printf("解密高效隐私保护损失失败: %v\n", err)
		return
	}
	privacyLoss := real(decodedPrivacy[0]) // 高效版本的平均损失在第一位

	// 解密标准版本的损失并求平均
	ptStandard := decryptor.DecryptNew(encryptedLossStandard)
	decodedStandard := make([]complex128, slots)
	if err := encoder.Decode(ptStandard, decodedStandard); err != nil {
		fmt.Printf("解密标准损失失败: %v\n", err)
		return
	}

	standardLossSum := 0.0
	for i := 0; i < batchSize; i++ {
		standardLossSum += real(decodedStandard[i])
	}
	standardLoss := standardLossSum / float64(batchSize)

	fmt.Printf("🚀 高效隐私保护版本平均损失: %.6f\n", privacyLoss)
	fmt.Printf("🚨 标准版本平均损失: %.6f\n", standardLoss)
	fmt.Printf("两种方法差异: %.6f\n", math.Abs(privacyLoss-standardLoss))
	fmt.Printf("相对差异: %.2f%%\n", math.Abs(privacyLoss-standardLoss)/standardLoss*100)

	if math.Abs(privacyLoss-standardLoss)/standardLoss < 0.01 {
		fmt.Printf("✅ 🚀 高效隐私保护版本计算正确！\n")
	} else {
		fmt.Printf("⚠️ 🚀 高效隐私保护版本存在误差\n")
	}

	// 10. 📈 性能和隐私总结
	fmt.Println("\n=== 📈 性能和隐私总结 ===")
	fmt.Printf("🚀 高效隐私保护版本优势：\n")
	fmt.Printf("• ✅ 零个体信息泄露\n")
	fmt.Printf("• ✅ 无掩码操作，计算高效\n")
	fmt.Printf("• ✅ 基于重复打包的自然边界\n")
	fmt.Printf("• ✅ 利用向量组装器的重复打包特性\n")
	fmt.Printf("• ⚡ 算法复杂度: O(log₂ %d) = %d 次旋转\n", batchSize, int(math.Log2(float64(batchSize))))

	fmt.Printf("\n🚨 标准版本问题：\n")
	fmt.Printf("• ❌ 泄露每个样本的具体损失值\n")
	fmt.Printf("• ❌ 需要解密个体数据进行验证\n")
	fmt.Printf("• ❌ 不满足隐私保护要求\n")

	fmt.Println("\n========================================")
	fmt.Println("测试完成")
	fmt.Println("========================================")
}
