package homomorphic

import (
	"MNIST-CNN/pkg/participant"
	"MNIST-CNN/pkg/protocols"
	"fmt"
	"math"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

// HomomorphicSoftmax 计算同态softmax
// neuronOutputs: 输出层每个神经元的输出密文
// 返回: softmax后的概率分布密文
func HomomorphicSoftmax(
	neuronOutputs []*rlwe.Ciphertext,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	slots int,
) ([]*rlwe.Ciphertext, error) {

	// 参数配置（优化后的参数）
	expK := 3.0       // 指数函数近似区间 [-K, K]（更保守的区间）
	expDegree := 20   // 指数函数多项式阶数（高阶多项式）
	recipK := 20.0    // 倒数函数近似区间上界（保守区间）
	recipEps := 1.0   // 倒数函数避免0的epsilon（避免极小值）
	recipDegree := 20 // 倒数函数多项式阶数（高阶多项式）

	// 创建激活函数
	expActivation := NewExpChebyshevActivation(expK, expDegree)
	recipActivation := NewReciprocalChebyshevActivation(recipK, recipEps, recipDegree)

	// 步骤1：对每个输出计算指数
	expOutputs := make([]*rlwe.Ciphertext, len(neuronOutputs))

	for i, output := range neuronOutputs {
		expResult, err := expActivation.Apply(output, evaluator, params)
		if err != nil {
			return nil, fmt.Errorf("计算exp失败(神经元%d): %v", i, err)
		}
		expOutputs[i] = expResult
	}

	// 步骤2：计算所有指数的和
	sumExp := expOutputs[0].CopyNew()

	for i := 1; i < len(expOutputs); i++ {
		var err error
		sumExp, err = evaluator.AddNew(sumExp, expOutputs[i])
		if err != nil {
			return nil, fmt.Errorf("累加exp失败: %v", err)
		}
	}

	// 步骤3：计算倒数 1/sum(exp)
	recipSum, err := recipActivation.Apply(sumExp, evaluator, params)
	if err != nil {
		return nil, fmt.Errorf("计算倒数失败: %v", err)
	}

	// 步骤4：计算softmax = exp(O_i) * (1/sum(exp))
	softmaxOutputs := make([]*rlwe.Ciphertext, len(expOutputs))

	for i, expOutput := range expOutputs {
		// 乘以倒数
		result, err := evaluator.MulNew(expOutput, recipSum)
		if err != nil {
			return nil, fmt.Errorf("计算softmax失败(神经元%d): %v", i, err)
		}

		// Rescale以保持精度
		if err := evaluator.Rescale(result, result); err != nil {
			return nil, fmt.Errorf("Rescale失败(神经元%d): %v", i, err)
		}

		softmaxOutputs[i] = result
	}

	// 检查并输出最终softmax密文的层级状态
	minFinalLevel := params.MaxLevel()
	for _, ct := range softmaxOutputs {
		if ct.Level() < minFinalLevel {
			minFinalLevel = ct.Level()
		}
	}

	fmt.Printf("✓ Softmax: %d个概率分布, 最低层级%d/%d\n",
		len(softmaxOutputs), minFinalLevel, params.MaxLevel())

	return softmaxOutputs, nil
}

// HomomorphicSoftmaxWithBootstrap 计算同态softmax（带关键步骤自举检查点）
func HomomorphicSoftmaxWithBootstrap(
	neuronOutputs []*rlwe.Ciphertext,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	slots int,
	// 自举相关参数
	N int,
	parties []*participant.Party,
	cloud *participant.Cloud,
	crs *sampling.KeyedPRNG,
) ([]*rlwe.Ciphertext, error) {

	fmt.Println("\n=== 计算同态Softmax（带自举检查点）===")

	// 设置自举阈值
	bootstrapThreshold := params.MaxLevel() - 3

	// 参数配置
	expK := 3.0       // 指数函数近似区间 [-K, K]
	expDegree := 20   // 指数函数多项式阶数
	recipK := 20.0    // 倒数函数近似区间上界
	recipEps := 1.0   // 倒数函数避免0的epsilon
	recipDegree := 20 // 倒数函数多项式阶数

	// 创建激活函数
	expActivation := NewExpChebyshevActivation(expK, expDegree)
	recipActivation := NewReciprocalChebyshevActivation(recipK, recipEps, recipDegree)

	// 检查输入层级
	fmt.Println("\n📍 输入密文层级检查：")
	minInputLevel := params.MaxLevel()
	for i, ct := range neuronOutputs {
		level := ct.Level()
		if level < minInputLevel {
			minInputLevel = level
		}
		fmt.Printf("  神经元%d: Level=%d/%d\n", i, level, params.MaxLevel())
	}
	fmt.Printf("  最低输入层级: %d/%d\n", minInputLevel, params.MaxLevel())

	// 🔍 输入检查：检查输入密文是否需要自举
	fmt.Printf("\n🔍 输入检查：检查是否需要对输入密文执行自举\n")
	if minInputLevel < bootstrapThreshold {
		fmt.Printf("\n🔄 输入检查触发：层级过低（%d < %d），对输入密文执行自举...\n", minInputLevel, bootstrapThreshold)

		// 转换为map格式
		inputMap := make(map[uint64]*rlwe.Ciphertext)
		for i, ct := range neuronOutputs {
			inputMap[uint64(i)] = ct
		}

		// 执行自举
		protocols.RefreshCiphertexts(params, N, parties, cloud, inputMap, crs)

		// 收集自举后的结果
		refreshedInputs := make([]*rlwe.Ciphertext, len(neuronOutputs))
		for i := 0; i < len(neuronOutputs); i++ {
			result := <-cloud.RefreshDone
			refreshedInputs[result.Key] = result.Ciphertext
			fmt.Printf("    神经元%d输入自举后层级: %d/%d\n", result.Key, result.Ciphertext.Level(), params.MaxLevel())
		}
		neuronOutputs = refreshedInputs
		fmt.Printf("✓ 输入检查自举完成\n")
	} else {
		fmt.Printf("✅ 输入检查通过：层级充足，无需自举\n")
	}

	// 步骤1：对每个输出计算指数
	fmt.Printf("\n📊 步骤1：计算exp(O_i)，共%d个神经元\n", len(neuronOutputs))
	expOutputs := make([]*rlwe.Ciphertext, len(neuronOutputs))

	for i, output := range neuronOutputs {
		fmt.Printf("  计算exp(神经元%d)...\n", i)
		expResult, err := expActivation.Apply(output, evaluator, params)
		if err != nil {
			return nil, fmt.Errorf("计算exp失败(神经元%d): %v", i, err)
		}
		expOutputs[i] = expResult
	}

	// 🔍 检查点1：指数计算后的层级检查
	fmt.Printf("\n🔍 检查点1：指数计算后层级检查\n")
	minExpLevel := params.MaxLevel()
	for i, ct := range expOutputs {
		level := ct.Level()
		if level < minExpLevel {
			minExpLevel = level
		}
		fmt.Printf("  神经元%d: exp后层级 %d/%d\n", i, level, params.MaxLevel())
	}
	fmt.Printf("  最低层级: %d/%d\n", minExpLevel, params.MaxLevel())

	if minExpLevel < bootstrapThreshold {
		fmt.Printf("\n🔄 检查点1触发：层级过低（%d < %d），对指数结果执行自举...\n", minExpLevel, bootstrapThreshold)

		// 转换为map格式
		expMap := make(map[uint64]*rlwe.Ciphertext)
		for i, ct := range expOutputs {
			expMap[uint64(i)] = ct
		}

		// 执行自举
		protocols.RefreshCiphertexts(params, N, parties, cloud, expMap, crs)

		// 收集自举后的结果
		refreshedExp := make([]*rlwe.Ciphertext, len(expOutputs))
		for i := 0; i < len(expOutputs); i++ {
			result := <-cloud.RefreshDone
			refreshedExp[result.Key] = result.Ciphertext
			fmt.Printf("    神经元%d自举后层级: %d/%d\n", result.Key, result.Ciphertext.Level(), params.MaxLevel())
		}
		expOutputs = refreshedExp
		fmt.Printf("✓ 检查点1自举完成\n")
	} else {
		fmt.Printf("✅ 检查点1通过：层级充足，无需自举\n")
	}

	// 步骤2：计算所有指数的和
	fmt.Println("\n📊 步骤2：计算sum(exp(O_i))")
	sumExp := expOutputs[0].CopyNew()

	for i := 1; i < len(expOutputs); i++ {
		fmt.Printf("  累加神经元%d的exp值...\n", i)
		var err error
		sumExp, err = evaluator.AddNew(sumExp, expOutputs[i])
		if err != nil {
			return nil, fmt.Errorf("累加exp失败: %v", err)
		}
	}

	// 步骤3：计算倒数 1/sum(exp)
	fmt.Println("\n📊 步骤3：计算1/sum(exp(O_i))")
	recipSum, err := recipActivation.Apply(sumExp, evaluator, params)
	if err != nil {
		return nil, fmt.Errorf("计算倒数失败: %v", err)
	}

	// 🔍 检查点2：倒数计算后的层级检查
	fmt.Printf("\n🔍 检查点2：倒数计算后层级检查\n")
	recipLevel := recipSum.Level()
	fmt.Printf("  倒数结果层级: %d/%d\n", recipLevel, params.MaxLevel())

	if recipLevel < bootstrapThreshold {
		fmt.Printf("\n🔄 检查点2触发：层级过低（%d < %d），对倒数结果执行自举...\n", recipLevel, bootstrapThreshold)

		// 对倒数结果执行自举
		recipMap := map[uint64]*rlwe.Ciphertext{0: recipSum}
		protocols.RefreshCiphertexts(params, N, parties, cloud, recipMap, crs)

		// 收集自举结果
		result := <-cloud.RefreshDone
		recipSum = result.Ciphertext
		fmt.Printf("✓ 检查点2自举完成，倒数新层级: %d/%d\n", recipSum.Level(), params.MaxLevel())
	} else {
		fmt.Printf("✅ 检查点2通过：层级充足，无需自举\n")
	}

	// 步骤4：计算softmax = exp(O_i) * (1/sum(exp))
	fmt.Println("\n📊 步骤4：计算softmax概率")
	softmaxOutputs := make([]*rlwe.Ciphertext, len(expOutputs))

	for i, expOutput := range expOutputs {
		fmt.Printf("  计算softmax(神经元%d) = exp(O_%d) / sum...\n", i, i)

		// 乘以倒数
		result, err := evaluator.MulNew(expOutput, recipSum)
		if err != nil {
			return nil, fmt.Errorf("计算softmax失败(神经元%d): %v", i, err)
		}

		// Rescale以保持精度
		if err := evaluator.Rescale(result, result); err != nil {
			return nil, fmt.Errorf("Rescale失败(神经元%d): %v", i, err)
		}

		softmaxOutputs[i] = result
	}

	// 🔍 检查点3：最终结果的层级检查
	fmt.Printf("\n🔍 检查点3：最终softmax结果层级检查\n")
	minFinalLevel := params.MaxLevel()
	for i, ct := range softmaxOutputs {
		level := ct.Level()
		if level < minFinalLevel {
			minFinalLevel = level
		}
		fmt.Printf("  神经元%d: 最终层级 %d/%d\n", i, level, params.MaxLevel())
	}
	fmt.Printf("  最低最终层级: %d/%d\n", minFinalLevel, params.MaxLevel())

	if minFinalLevel < bootstrapThreshold {
		fmt.Printf("\n🔄 检查点3触发：层级过低（%d < %d），对最终结果执行自举...\n", minFinalLevel, bootstrapThreshold)

		// 将最终结果转换为map格式
		finalMap := make(map[uint64]*rlwe.Ciphertext)
		for i, ct := range softmaxOutputs {
			finalMap[uint64(i)] = ct
		}

		// 执行自举
		protocols.RefreshCiphertexts(params, N, parties, cloud, finalMap, crs)

		// 收集自举后的结果
		refreshedFinal := make([]*rlwe.Ciphertext, len(softmaxOutputs))
		for i := 0; i < len(softmaxOutputs); i++ {
			result := <-cloud.RefreshDone
			refreshedFinal[result.Key] = result.Ciphertext
			fmt.Printf("    神经元%d最终自举后层级: %d/%d\n", result.Key, result.Ciphertext.Level(), params.MaxLevel())
		}
		softmaxOutputs = refreshedFinal
		fmt.Printf("✓ 检查点3自举完成\n")
	} else {
		fmt.Printf("✅ 检查点3通过：层级充足，无需自举\n")
	}

	// 最终层级状态显示
	fmt.Printf("\n🔍 最终Softmax密文层级状态：\n")
	fmt.Println("序号 | 层级     | 缩放比例        | 状态")
	fmt.Println("-----|----------|----------------|----------")

	finalMinLevel := params.MaxLevel()
	totalLevels := 0

	for i, ct := range softmaxOutputs {
		level := ct.Level()
		scale := ct.Scale.Log2()

		if level < finalMinLevel {
			finalMinLevel = level
		}
		totalLevels += level

		// 计算层级状态
		ratio := float64(level) / float64(params.MaxLevel())
		var status string
		if ratio > 0.6 {
			status = "充足 ✅"
		} else if ratio > 0.3 {
			status = "中等 ⚠️"
		} else {
			status = "偏低 ❌"
		}

		fmt.Printf("%4d | %3d/%-3d | 2^%8.2f | %s\n",
			i, level, params.MaxLevel(), scale, status)
	}

	avgLevel := float64(totalLevels) / float64(len(softmaxOutputs))

	fmt.Println("-----|----------|----------------|----------")
	fmt.Printf("统计 | 最低: %d | 平均: %.1f | 最大: %d\n", finalMinLevel, avgLevel, params.MaxLevel())

	// 给出层级评估
	if finalMinLevel < params.MaxLevel()/3 {
		fmt.Printf("⚠️ 警告：最低层级过低（%d/%d），后续计算可能需要自举\n", finalMinLevel, params.MaxLevel())
	} else if finalMinLevel < params.MaxLevel()/2 {
		fmt.Printf("💡 提示：层级中等（%d/%d），建议监控后续计算\n", finalMinLevel, params.MaxLevel())
	} else {
		fmt.Printf("✅ 良好：层级充足（%d/%d），可继续后续计算\n", finalMinLevel, params.MaxLevel())
	}

	fmt.Printf("\n✅ Softmax计算完成，生成%d个概率分布\n", len(softmaxOutputs))
	fmt.Printf("  📈 参数设置：exp区间[-%g, %g], exp阶数=%d, recip区间[%g, %g], recip阶数=%d\n",
		expK, expK, expDegree, recipEps, recipK, recipDegree)
	fmt.Printf("  🎯 自举阈值: %d/%d\n", bootstrapThreshold, params.MaxLevel())

	return softmaxOutputs, nil
}

// VerifySoftmax 验证softmax计算结果
func VerifySoftmax(
	originalOutputs []*rlwe.Ciphertext,
	softmaxOutputs []*rlwe.Ciphertext,
	decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder,
	slots int,
	n int, // 批次大小
) {
	fmt.Println("\n=== 验证Softmax结果 ===")

	// 解密原始输出
	fmt.Println("\n解密原始输出...")
	originalValues := make([][]float64, len(originalOutputs))
	for i, ct := range originalOutputs {
		pt := decryptor.DecryptNew(ct)
		decoded := make([]complex128, slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密原始输出%d失败: %v\n", i, err)
			continue
		}

		originalValues[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			originalValues[i][j] = real(decoded[j])
		}
	}

	// 解密softmax输出
	fmt.Println("解密softmax输出...")
	softmaxValues := make([][]float64, len(softmaxOutputs))
	for i, ct := range softmaxOutputs {
		pt := decryptor.DecryptNew(ct)
		decoded := make([]complex128, slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密softmax输出%d失败: %v\n", i, err)
			continue
		}

		softmaxValues[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			softmaxValues[i][j] = real(decoded[j])
		}
	}

	// 计算明文softmax作为对比
	fmt.Println("\n计算明文softmax...")
	plaintextSoftmax := make([][]float64, n)
	for sampleIdx := 0; sampleIdx < n; sampleIdx++ {
		plaintextSoftmax[sampleIdx] = make([]float64, len(originalOutputs))

		// 计算exp
		expSum := 0.0
		expValues := make([]float64, len(originalOutputs))
		for neuronIdx := 0; neuronIdx < len(originalOutputs); neuronIdx++ {
			expValues[neuronIdx] = math.Exp(originalValues[neuronIdx][sampleIdx])
			expSum += expValues[neuronIdx]
		}

		// 计算softmax
		for neuronIdx := 0; neuronIdx < len(originalOutputs); neuronIdx++ {
			plaintextSoftmax[sampleIdx][neuronIdx] = expValues[neuronIdx] / expSum
		}
	}

	// 比较结果
	fmt.Println("\n对比前5个样本的softmax结果：")
	fmt.Println("样本 | 类别 | 原始值 | 明文Softmax | 密文Softmax | 绝对误差")
	fmt.Println("-----|------|--------|-------------|-------------|----------")

	maxError := 0.0
	avgError := 0.0
	errorCount := 0

	for sampleIdx := 0; sampleIdx < min(5, n); sampleIdx++ {
		for neuronIdx := 0; neuronIdx < min(3, len(originalOutputs)); neuronIdx++ {
			original := originalValues[neuronIdx][sampleIdx]
			plainSoftmax := plaintextSoftmax[sampleIdx][neuronIdx]
			encSoftmax := softmaxValues[neuronIdx][sampleIdx]

			absError := math.Abs(plainSoftmax - encSoftmax)
			maxError = math.Max(maxError, absError)
			avgError += absError
			errorCount++

			fmt.Printf("%4d | %4d | %6.3f | %11.8f | %11.8f | %9.3e %s\n",
				sampleIdx, neuronIdx,
				original, plainSoftmax, encSoftmax,
				absError,
				func() string {
					if absError < 1e-6 {
						return "✓"
					} else if absError < 1e-3 {
						return "⚠"
					}
					return "❌"
				}())
		}
		if sampleIdx < min(5, n)-1 {
			fmt.Println("-----|------|--------|-------------|-------------|----------")
		}
	}

	// 验证概率和是否为1
	fmt.Println("\n验证概率和（前5个样本）：")
	fmt.Println("样本 | 明文和 | 密文和 | 误差")
	fmt.Println("-----|--------|--------|--------")

	for sampleIdx := 0; sampleIdx < min(5, n); sampleIdx++ {
		plainSum := 0.0
		encSum := 0.0

		for neuronIdx := 0; neuronIdx < len(originalOutputs); neuronIdx++ {
			plainSum += plaintextSoftmax[sampleIdx][neuronIdx]
			encSum += softmaxValues[neuronIdx][sampleIdx]
		}

		sumError := math.Abs(plainSum - encSum)
		fmt.Printf("%4d | %6.4f | %6.4f | %6.3e %s\n",
			sampleIdx, plainSum, encSum, sumError,
			func() string {
				if math.Abs(encSum-1.0) < 1e-3 {
					return "✓"
				}
				return "⚠"
			}())
	}

	// 统计信息
	if errorCount > 0 {
		avgError /= float64(errorCount)
	}

	fmt.Println("\n=== 误差统计 ===")
	fmt.Printf("最大绝对误差: %.3e\n", maxError)
	fmt.Printf("平均绝对误差: %.3e\n", avgError)

	if maxError < 1e-3 {
		fmt.Println("\n✓ Softmax计算验证通过！")
	} else {
		fmt.Println("\n⚠ Softmax计算存在一定误差，可能需要调整参数")
	}
}
