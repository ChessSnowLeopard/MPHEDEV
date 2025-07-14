package homomorphic

import (
	"fmt"
	"math"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// OutputGradientEngine 输出层梯度计算引擎
type OutputGradientEngine struct {
	NumClasses int // 输出类别数 (t'')
	BatchSize  int // 批大小 (s/k)
	Slots      int // 密文槽数 (s)
}

// NewOutputGradientEngine 创建输出层梯度计算引擎
func NewOutputGradientEngine(numClasses, batchSize, slots int) *OutputGradientEngine {
	return &OutputGradientEngine{
		NumClasses: numClasses,
		BatchSize:  batchSize,
		Slots:      slots,
	}
}

// ComputeOutputLayerGradients 计算输出层梯度（使用密文标签）
// 根据公式：δ_m^j = p_m^j - y_m^j
func (oge *OutputGradientEngine) ComputeOutputLayerGradients(
	softmaxOutputs []*rlwe.Ciphertext, // P^j: Softmax概率输出
	encryptedLabels map[uint64]*rlwe.Ciphertext, // 密文one-hot编码标签
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
) ([]*rlwe.Ciphertext, error) {

	fmt.Println("\n=== 🎯 输出层梯度计算 (δ = p - y) [密文标签版] ===")
	fmt.Printf("类别数: %d, 批大小: %d\n", oge.NumClasses, oge.BatchSize)

	if len(softmaxOutputs) != oge.NumClasses {
		return nil, fmt.Errorf("softmax输出数量 (%d) 与类别数 (%d) 不匹配", len(softmaxOutputs), oge.NumClasses)
	}

	if len(encryptedLabels) != oge.NumClasses {
		return nil, fmt.Errorf("密文标签数量 (%d) 与类别数 (%d) 不匹配", len(encryptedLabels), oge.NumClasses)
	}

	// 计算梯度 δ^j = P^j - Y^j（直接使用密文标签）
	fmt.Println("\n📊 步骤1：计算输出层梯度 δ^j = P^j - Y^j（密文vs密文）")
	outputGradients := make([]*rlwe.Ciphertext, oge.NumClasses)

	for j := 0; j < oge.NumClasses; j++ {
		fmt.Printf("  计算类别%d的梯度...\n", j)

		// 获取对应类别的密文标签
		encryptedLabel, exists := encryptedLabels[uint64(j)]
		if !exists {
			return nil, fmt.Errorf("找不到类别%d的密文标签", j)
		}

		// δ^j = P^j - Y^j (密文 - 密文)
		gradient, err := evaluator.SubNew(softmaxOutputs[j], encryptedLabel)
		if err != nil {
			return nil, fmt.Errorf("计算梯度失败(类别%d): %v", j, err)
		}

		outputGradients[j] = gradient
		fmt.Printf("    梯度层级: %d/%d\n", gradient.Level(), params.MaxLevel())
	}

	fmt.Printf("✅ 输出层梯度计算完成\n")
	return outputGradients, nil
}

// createOneHotLabels 创建one-hot编码的标签明文
func (oge *OutputGradientEngine) createOneHotLabels(labels []byte, params ckks.Parameters, encoder *ckks.Encoder) ([]*rlwe.Plaintext, error) {
	oneHotLabels := make([]*rlwe.Plaintext, oge.NumClasses)

	for j := 0; j < oge.NumClasses; j++ {
		labelVector := make([]complex128, oge.Slots)

		// 为每个样本设置one-hot编码
		for m := 0; m < oge.BatchSize; m++ {
			var labelValue float64
			if int(labels[m]) == j {
				labelValue = 1.0 // 真实类别
			} else {
				labelValue = 0.0 // 非真实类别
			}

			// 由于打包格式，每个段都填入相同的值
			for seg := 0; seg < oge.Slots/oge.BatchSize; seg++ {
				startIdx := seg * oge.BatchSize
				if startIdx+m < oge.Slots {
					labelVector[startIdx+m] = complex(labelValue, 0)
				}
			}
		}

		// 编码为明文
		pt := ckks.NewPlaintext(params, params.MaxLevel())
		if err := encoder.Encode(labelVector, pt); err != nil {
			return nil, fmt.Errorf("编码标签失败(类别%d): %v", j, err)
		}

		oneHotLabels[j] = pt
		fmt.Printf("  类别%d标签编码完成\n", j)
	}

	return oneHotLabels, nil
}

// VerifyOutputLayerGradients 验证输出层梯度计算精度（密文标签版）
func (oge *OutputGradientEngine) VerifyOutputLayerGradients(
	outputGradients []*rlwe.Ciphertext,
	softmaxOutputs []*rlwe.Ciphertext,
	encryptedLabels map[uint64]*rlwe.Ciphertext,
	decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder,
) {
	fmt.Println("\n=== 🔍 验证输出层梯度计算精度 [密文标签版] ===")

	// 解密密文梯度
	encryptedGradients := make([][]float64, oge.NumClasses)
	for j := 0; j < oge.NumClasses; j++ {
		pt := decryptor.DecryptNew(outputGradients[j])
		decoded := make([]complex128, oge.Slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密梯度失败(类别%d): %v\n", j, err)
			continue
		}

		encryptedGradients[j] = make([]float64, oge.BatchSize)
		for i := 0; i < oge.BatchSize; i++ {
			encryptedGradients[j][i] = real(decoded[i])
		}
	}

	// 解密softmax输出
	softmaxValues := make([][]float64, oge.NumClasses)
	for j := 0; j < oge.NumClasses; j++ {
		pt := decryptor.DecryptNew(softmaxOutputs[j])
		decoded := make([]complex128, oge.Slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密softmax输出失败(类别%d): %v\n", j, err)
			continue
		}

		softmaxValues[j] = make([]float64, oge.BatchSize)
		for i := 0; i < oge.BatchSize; i++ {
			softmaxValues[j][i] = real(decoded[i])
		}
	}

	// 解密密文标签
	labelValues := make([][]float64, oge.NumClasses)
	for j := 0; j < oge.NumClasses; j++ {
		if encryptedLabel, exists := encryptedLabels[uint64(j)]; exists {
			pt := decryptor.DecryptNew(encryptedLabel)
			decoded := make([]complex128, oge.Slots)
			if err := encoder.Decode(pt, decoded); err != nil {
				fmt.Printf("解密密文标签失败(类别%d): %v\n", j, err)
				continue
			}

			labelValues[j] = make([]float64, oge.BatchSize)
			for i := 0; i < oge.BatchSize; i++ {
				labelValues[j][i] = real(decoded[i])
			}
		}
	}

	// 计算明文梯度进行对比
	fmt.Println("\n📊 梯度精度对比（前5个样本）：")
	fmt.Println("样本 | 类别 | 标签值 | Softmax值 | 明文梯度 | 密文梯度 | 绝对误差 | 相对误差")
	fmt.Println("-----|------|--------|-----------|----------|----------|----------|----------")

	totalError := 0.0
	maxError := 0.0
	errorCount := 0

	for m := 0; m < min(5, oge.BatchSize); m++ {
		for j := 0; j < oge.NumClasses; j++ {
			// 计算明文梯度: δ_m^j = p_m^j - y_m^j
			pValue := softmaxValues[j][m]
			yValue := labelValues[j][m]
			plaintextGradient := pValue - yValue

			// 获取密文梯度
			ciphertextGradient := encryptedGradients[j][m]

			// 计算误差
			absError := math.Abs(plaintextGradient - ciphertextGradient)
			var relError float64
			if math.Abs(plaintextGradient) > 1e-10 {
				relError = absError / math.Abs(plaintextGradient) * 100
			} else {
				relError = 0
			}

			totalError += absError
			if absError > maxError {
				maxError = absError
			}
			errorCount++

			// 标记显示（如果是正确类别）
			var marker string
			if yValue > 0.5 {
				marker = "✓"
			} else {
				marker = " "
			}

			fmt.Printf("%4d | %4d | %6.3f | %9.6f | %8.6f | %8.6f | %8.6f | %7.3f%% %s\n",
				m, j, yValue, pValue, plaintextGradient, ciphertextGradient, absError, relError, marker)
		}
	}

	// 统计信息
	avgError := totalError / float64(errorCount)

	fmt.Printf("\n=== 精度统计 ===\n")
	fmt.Printf("• 平均绝对误差: %.8f\n", avgError)
	fmt.Printf("• 最大绝对误差: %.8f\n", maxError)
	fmt.Printf("• 总样本-类别对数: %d\n", errorCount)

	// 评估精度
	if maxError < 1e-6 {
		fmt.Printf("✅ 输出层梯度验证通过！(误差 < 1e-6)\n")
	} else if maxError < 1e-3 {
		fmt.Printf("⚠️ 输出层梯度精度可接受 (1e-6 < 误差 < 1e-3)\n")
	} else {
		fmt.Printf("❌ 输出层梯度精度不足 (误差 > 1e-3)\n")
	}

	// 显示梯度特性
	fmt.Printf("\n📈 梯度特性分析:\n")
	for m := 0; m < min(3, oge.BatchSize); m++ {
		fmt.Printf("  样本%d:\n", m)

		gradientSum := 0.0
		for j := 0; j < oge.NumClasses; j++ {
			gradient := encryptedGradients[j][m]
			gradientSum += gradient
			label := labelValues[j][m]
			if label > 0.5 {
				fmt.Printf("    类别%d: %8.6f ← 真实类别\n", j, gradient)
			} else {
				fmt.Printf("    类别%d: %8.6f\n", j, gradient)
			}
		}
		fmt.Printf("    梯度和: %8.6f (理论值: 0)\n", gradientSum)
	}
}

// ComputePlaintextGradients 计算明文梯度用于对比验证
func (oge *OutputGradientEngine) ComputePlaintextGradients(
	softmaxValues [][]float64, // [类别][样本]的softmax值
	labels []byte,
) [][]float64 {
	fmt.Println("\n📊 计算明文梯度用于对比")

	gradients := make([][]float64, oge.NumClasses)
	for j := 0; j < oge.NumClasses; j++ {
		gradients[j] = make([]float64, oge.BatchSize)
	}

	for m := 0; m < oge.BatchSize && m < len(labels); m++ {
		trueLabel := int(labels[m])

		for j := 0; j < oge.NumClasses; j++ {
			// δ_m^j = p_m^j - y_m^j
			var yValue float64
			if j == trueLabel {
				yValue = 1.0
			} else {
				yValue = 0.0
			}

			gradients[j][m] = softmaxValues[j][m] - yValue
		}
	}

	return gradients
}

// ComputeOutputWeightGradients 计算输出层权重梯度
// 根据公式：∂L̄/∂W_{j,l} = (1/(s/k)) × Σ_{m=0}^{s/k-1} δ_m^j × A_m^l
func (oge *OutputGradientEngine) ComputeOutputWeightGradients(
	outputGradients []*rlwe.Ciphertext, // Δ^j: 输出层梯度
	hiddenActivations []*rlwe.Ciphertext, // A^{group}: 隐藏层激活（重组后）
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
) ([][]*rlwe.Ciphertext, error) {

	fmt.Println("\n=== 🎯 计算输出层权重梯度 ===")
	fmt.Printf("输出神经元数: %d, 隐藏层组数: %d, 批大小: %d\n",
		len(outputGradients), len(hiddenActivations), oge.BatchSize)

	// 输入验证
	if len(outputGradients) == 0 {
		return nil, fmt.Errorf("输出层梯度为空")
	}
	if len(hiddenActivations) == 0 {
		return nil, fmt.Errorf("隐藏层激活为空")
	}
	if len(outputGradients) != oge.NumClasses {
		return nil, fmt.Errorf("输出梯度数量 (%d) 与类别数 (%d) 不匹配",
			len(outputGradients), oge.NumClasses)
	}

	// 检查层级是否足够
	for j, ct := range outputGradients {
		if ct.Level() < 2 {
			return nil, fmt.Errorf("输出梯度%d层级不足 (%d < 2)，需要自举", j, ct.Level())
		}
	}

	numOutputNeurons := len(outputGradients)
	numHiddenGroups := len(hiddenActivations)

	// 初始化权重梯度存储
	weightGradients := make([][]*rlwe.Ciphertext, numOutputNeurons)
	for j := 0; j < numOutputNeurons; j++ {
		weightGradients[j] = make([]*rlwe.Ciphertext, numHiddenGroups)
	}

	// 对每个输出神经元计算权重梯度
	for j := 0; j < numOutputNeurons; j++ {
		fmt.Printf("\n📊 计算输出神经元%d的权重梯度...\n", j)

		// 对每个隐藏层权重组计算梯度
		for groupIdx := 0; groupIdx < numHiddenGroups; groupIdx++ {
			fmt.Printf("  处理权重组%d/%d...\n", groupIdx+1, numHiddenGroups)

			// 🔧 智能degree管理：检查并自动重线性化
			outputCt := outputGradients[j]
			hiddenCt := hiddenActivations[groupIdx]

			outputDegree := outputCt.Degree()
			hiddenDegree := hiddenCt.Degree()
			totalDegree := outputDegree + hiddenDegree

			fmt.Printf("    degree检查: 输出度=%d, 隐藏度=%d, 总度=%d\n",
				outputDegree, hiddenDegree, totalDegree)

			// 如果总度过高，对degree高的密文进行重线性化
			if totalDegree > 2 {
				fmt.Printf("    ⚠️ 总度过高(%d > 2)，执行预处理重线性化...\n", totalDegree)

				// 对degree更高的密文进行重线性化
				if outputDegree > 1 {
					fmt.Printf("      重线性化输出梯度(度: %d -> 1)...\n", outputDegree)
					if err := evaluator.Relinearize(outputCt, outputCt); err != nil {
						return nil, fmt.Errorf("重线性化输出梯度失败(神经元%d): %v", j, err)
					}
					fmt.Printf("      ✅ 输出梯度重线性化完成，新度: %d\n", outputCt.Degree())
				}

				if hiddenCt.Degree() > 1 {
					fmt.Printf("      重线性化隐藏激活(度: %d -> 1)...\n", hiddenCt.Degree())
					if err := evaluator.Relinearize(hiddenCt, hiddenCt); err != nil {
						return nil, fmt.Errorf("重线性化隐藏激活失败(组%d): %v", groupIdx, err)
					}
					fmt.Printf("      ✅ 隐藏激活重线性化完成，新度: %d\n", hiddenCt.Degree())
				}

				// 重新计算度
				outputDegree = outputCt.Degree()
				hiddenDegree = hiddenCt.Degree()
				totalDegree = outputDegree + hiddenDegree
				fmt.Printf("    ✅ 重线性化后度: 输出度=%d, 隐藏度=%d, 总度=%d\n",
					outputDegree, hiddenDegree, totalDegree)
			}

			// 步骤1: 计算梯度乘积 G = Δ^j ⊙ A^{group}
			fmt.Printf("    步骤1: 计算梯度乘积 Δ^%d ⊙ A^{group%d} (度: %d+%d=%d)\n",
				j, groupIdx, outputDegree, hiddenDegree, totalDegree)
			gradientProduct, err := evaluator.MulNew(outputCt, hiddenCt)
			if err != nil {
				return nil, fmt.Errorf("计算梯度乘积失败(神经元%d, 组%d): %v", j, groupIdx, err)
			}

			// 重要：乘法后需要重线性化和Rescale
			if err := evaluator.Relinearize(gradientProduct, gradientProduct); err != nil {
				return nil, fmt.Errorf("重线性化失败(神经元%d, 组%d): %v", j, groupIdx, err)
			}
			if err := evaluator.Rescale(gradientProduct, gradientProduct); err != nil {
				return nil, fmt.Errorf("Rescale失败(神经元%d, 组%d): %v", j, groupIdx, err)
			}

			fmt.Printf("    乘积计算完成，层级: %d/%d\n", gradientProduct.Level(), params.MaxLevel())

			// 步骤2: 段内聚合求和
			fmt.Printf("    步骤2: 段内聚合求和\n")
			aggregatedGradient, err := oge.performSegmentAggregation(gradientProduct, oge.BatchSize, evaluator)
			if err != nil {
				return nil, fmt.Errorf("段内聚合失败(神经元%d, 组%d): %v", j, groupIdx, err)
			}

			// 步骤3: 除以批大小求平均
			fmt.Printf("    步骤3: 计算平均梯度 (÷%d)\n", oge.BatchSize)
			avgScale := 1.0 / float64(oge.BatchSize)

			// 使用MulRelin确保乘法后自动rescale
			if err := evaluator.MulRelin(aggregatedGradient, avgScale, aggregatedGradient); err != nil {
				return nil, fmt.Errorf("计算平均梯度失败(神经元%d, 组%d): %v", j, groupIdx, err)
			}

			weightGradients[j][groupIdx] = aggregatedGradient
			fmt.Printf("    ✅ 权重组%d梯度计算完成，最终层级: %d/%d\n",
				groupIdx, aggregatedGradient.Level(), params.MaxLevel())
		}
	}

	fmt.Printf("\n✅ 输出层权重梯度计算完成！\n")
	fmt.Printf("• 每个梯度格式: [∇W_{j,l}, ∇W_{j,l}, ..., ∇W_{j,l} | 重复段 | ...]\n")

	return weightGradients, nil
}

// performSegmentAggregation 执行段内聚合求和
// 输入格式: [v_0, v_1, ..., v_{batchSize-1} | v_0, v_1, ..., v_{batchSize-1} | ...]
// 输出格式: [Σv_i, Σv_i, ..., Σv_i | Σv_i, Σv_i, ..., Σv_i | ...]
func (oge *OutputGradientEngine) performSegmentAggregation(
	ciphertext *rlwe.Ciphertext,
	batchSize int,
	evaluator *ckks.Evaluator,
) (*rlwe.Ciphertext, error) {

	if batchSize <= 1 {
		return ciphertext, nil
	}

	fmt.Printf("      🔄 开始段内聚合（批大小: %d）...\n", batchSize)

	// 检查是否为2的幂次，决定使用哪种聚合方法
	if isPowerOfTwo(batchSize) {
		fmt.Printf("      使用对数级聚合（%d轮）\n", logBase2(batchSize))
		return oge.logarithmicSegmentAggregation(ciphertext, batchSize, evaluator)
	} else {
		fmt.Printf("      使用线性聚合（%d次旋转）\n", batchSize-1)
		return oge.linearSegmentAggregation(ciphertext, batchSize, evaluator)
	}
}

// logarithmicSegmentAggregation 对数级段内聚合（当batchSize为2的幂次时）
func (oge *OutputGradientEngine) logarithmicSegmentAggregation(
	ciphertext *rlwe.Ciphertext,
	batchSize int,
	evaluator *ckks.Evaluator,
) (*rlwe.Ciphertext, error) {

	result := ciphertext.CopyNew()
	rounds := logBase2(batchSize)

	for round := 0; round < rounds; round++ {
		rotateBy := 1 << round // 1, 2, 4, 8, ...

		fmt.Printf("        轮次%d: 旋转%d位\n", round+1, rotateBy)

		// 旋转并累加
		rotated, err := evaluator.RotateNew(result, rotateBy)
		if err != nil {
			return nil, fmt.Errorf("旋转失败(轮次%d): %v", round+1, err)
		}

		result, err = evaluator.AddNew(result, rotated)
		if err != nil {
			return nil, fmt.Errorf("累加失败(轮次%d): %v", round+1, err)
		}
	}

	fmt.Printf("      ✅ 对数级聚合完成\n")
	return result, nil
}

// linearSegmentAggregation 线性段内聚合（当batchSize不是2的幂次时）
func (oge *OutputGradientEngine) linearSegmentAggregation(
	ciphertext *rlwe.Ciphertext,
	batchSize int,
	evaluator *ckks.Evaluator,
) (*rlwe.Ciphertext, error) {

	result := ciphertext.CopyNew()

	for i := 1; i < batchSize; i++ {
		fmt.Printf("        步骤%d: 旋转%d位\n", i, i)

		// 旋转并累加
		rotated, err := evaluator.RotateNew(ciphertext, i)
		if err != nil {
			return nil, fmt.Errorf("旋转失败(步骤%d): %v", i, err)
		}

		result, err = evaluator.AddNew(result, rotated)
		if err != nil {
			return nil, fmt.Errorf("累加失败(步骤%d): %v", i, err)
		}
	}

	fmt.Printf("      ✅ 线性聚合完成\n")
	return result, nil
}

// VerifyOutputWeightGradients 验证输出层权重梯度计算的正确性
func (oge *OutputGradientEngine) VerifyOutputWeightGradients(
	weightGradients [][]*rlwe.Ciphertext,
	outputGradients []*rlwe.Ciphertext,
	hiddenActivations []*rlwe.Ciphertext,
	decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder,
) {
	fmt.Println("\n=== 🔍 验证输出层权重梯度 ===")

	numOutputNeurons := len(weightGradients)
	numHiddenGroups := len(weightGradients[0])

	// 解密权重梯度
	decryptedWeightGradients := make([][][]float64, numOutputNeurons)
	for j := 0; j < numOutputNeurons; j++ {
		decryptedWeightGradients[j] = make([][]float64, numHiddenGroups)
		for groupIdx := 0; groupIdx < numHiddenGroups; groupIdx++ {
			pt := decryptor.DecryptNew(weightGradients[j][groupIdx])
			decoded := make([]complex128, oge.Slots)
			if err := encoder.Decode(pt, decoded); err != nil {
				fmt.Printf("解密权重梯度失败(神经元%d, 组%d): %v\n", j, groupIdx, err)
				continue
			}

			decryptedWeightGradients[j][groupIdx] = make([]float64, oge.BatchSize)
			for i := 0; i < oge.BatchSize; i++ {
				decryptedWeightGradients[j][groupIdx][i] = real(decoded[i])
			}
		}
	}

	// 解密输出层梯度和隐藏层激活
	decryptedOutputGradients := make([][]float64, numOutputNeurons)
	for j := 0; j < numOutputNeurons; j++ {
		pt := decryptor.DecryptNew(outputGradients[j])
		decoded := make([]complex128, oge.Slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密输出梯度失败(神经元%d): %v\n", j, err)
			continue
		}

		decryptedOutputGradients[j] = make([]float64, oge.BatchSize)
		for i := 0; i < oge.BatchSize; i++ {
			decryptedOutputGradients[j][i] = real(decoded[i])
		}
	}

	decryptedHiddenActivations := make([][]float64, numHiddenGroups)
	for groupIdx := 0; groupIdx < numHiddenGroups; groupIdx++ {
		pt := decryptor.DecryptNew(hiddenActivations[groupIdx])
		decoded := make([]complex128, oge.Slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密隐藏激活失败(组%d): %v\n", groupIdx, err)
			continue
		}

		decryptedHiddenActivations[groupIdx] = make([]float64, oge.BatchSize)
		for i := 0; i < oge.BatchSize; i++ {
			decryptedHiddenActivations[groupIdx][i] = real(decoded[i])
		}
	}

	// 计算明文权重梯度进行对比
	fmt.Println("\n📊 权重梯度精度对比（前3个神经元和组）：")
	fmt.Println("神经元 | 组 | 明文梯度 | 密文梯度 | 绝对误差 | 相对误差")
	fmt.Println("-------|----|---------|---------|---------|---------")

	totalError := 0.0
	maxError := 0.0
	errorCount := 0

	for j := 0; j < min(3, numOutputNeurons); j++ {
		for groupIdx := 0; groupIdx < min(2, numHiddenGroups); groupIdx++ {
			// 计算明文权重梯度: ∂L̄/∂W_{j,group} = (1/batchSize) × Σ δ_m^j × A_m^{group}
			plaintextGradientSum := 0.0
			for m := 0; m < oge.BatchSize; m++ {
				plaintextGradientSum += decryptedOutputGradients[j][m] * decryptedHiddenActivations[groupIdx][m]
			}
			plaintextGradient := plaintextGradientSum / float64(oge.BatchSize)

			// 获取密文权重梯度（取第一个位置的值，因为聚合后所有位置都相同）
			ciphertextGradient := decryptedWeightGradients[j][groupIdx][0]

			// 计算误差
			absError := math.Abs(plaintextGradient - ciphertextGradient)
			var relError float64
			if math.Abs(plaintextGradient) > 1e-10 {
				relError = absError / math.Abs(plaintextGradient) * 100
			} else {
				relError = 0
			}

			totalError += absError
			if absError > maxError {
				maxError = absError
			}
			errorCount++

			fmt.Printf("%6d | %2d | %8.6f | %8.6f | %8.3e | %7.3f%%\n",
				j, groupIdx, plaintextGradient, ciphertextGradient, absError, relError)
		}
	}

	// 统计信息
	if errorCount > 0 {
		avgError := totalError / float64(errorCount)

		fmt.Printf("\n=== 精度统计 ===\n")
		fmt.Printf("• 平均绝对误差: %.8f\n", avgError)
		fmt.Printf("• 最大绝对误差: %.8f\n", maxError)
		fmt.Printf("• 验证的权重数: %d\n", errorCount)

		// 评估精度
		if maxError < 1e-6 {
			fmt.Printf("✅ 输出层权重梯度计算正确！(误差 < 1e-6)\n")
		} else if maxError < 1e-3 {
			fmt.Printf("⚠️ 权重梯度误差在可接受范围内 (1e-6 < 误差 < 1e-3)\n")
		} else {
			fmt.Printf("❌ 权重梯度误差过大 (误差 > 1e-3)\n")
		}
	}
}

// logBase2 计算以2为底的对数
func logBase2(n int) int {
	count := 0
	for n > 1 {
		n >>= 1
		count++
	}
	return count
}

// isPowerOfTwo 检查一个数是否为2的幂次
func isPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}
