package homomorphic

import (
	"fmt"
	"math"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// HiddenWeightGradientEngine 隐藏层权重梯度计算引擎
// 基于神经元格式的高效实现
type HiddenWeightGradientEngine struct {
	HiddenSize     int // 隐藏层神经元数 (t')
	InputSize      int // 输入特征数 (t)
	BatchSize      int // 批大小 (s/k)
	Slots          int // 密文槽数 (s)
	K              int // 每密文特征数
	NumInputBlocks int // 输入特征块数 (t/k)
}

// NewHiddenWeightGradientEngine 创建隐藏层权重梯度计算引擎
func NewHiddenWeightGradientEngine(hiddenSize, inputSize, batchSize, slots, k int) *HiddenWeightGradientEngine {
	numInputBlocks := (inputSize + k - 1) / k // 向上取整
	return &HiddenWeightGradientEngine{
		HiddenSize:     hiddenSize,
		InputSize:      inputSize,
		BatchSize:      batchSize,
		Slots:          slots,
		K:              k,
		NumInputBlocks: numInputBlocks,
	}
}

// ComputeHiddenWeightGradients 计算隐藏层权重梯度
// 根据公式：∇W_{i,f} = (1/batchSize) × Σ_{m=0}^{batchSize-1} δ_{hidden,m}^i × X_m[f]
func (hwge *HiddenWeightGradientEngine) ComputeHiddenWeightGradients(
	hiddenGradients []*rlwe.Ciphertext, // 隐藏层梯度：16个密文，每个神经元一个
	inputFeatures map[uint64]*rlwe.Ciphertext, // 输入特征：按块组织的密文
	batchIdx int, // 批次索引
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
) ([][]*rlwe.Ciphertext, error) {

	fmt.Println("\n=== 🎯 计算隐藏层权重梯度 ===")
	fmt.Printf("隐藏神经元数: %d, 输入特征数: %d, 特征块数: %d\n",
		hwge.HiddenSize, hwge.InputSize, hwge.NumInputBlocks)
	fmt.Printf("批次大小: %d, 批次索引: %d\n", hwge.BatchSize, batchIdx)

	startTime := time.Now()

	// 输入验证
	if len(hiddenGradients) != hwge.HiddenSize {
		return nil, fmt.Errorf("隐藏层梯度数量 (%d) 与隐藏层大小 (%d) 不匹配",
			len(hiddenGradients), hwge.HiddenSize)
	}

	// 计算该批次对应的输入特征密文索引范围
	startVecIdx := batchIdx * hwge.NumInputBlocks

	// 验证输入特征是否存在
	for blockIdx := 0; blockIdx < hwge.NumInputBlocks; blockIdx++ {
		vecIdx := uint64(startVecIdx + blockIdx)
		if _, exists := inputFeatures[vecIdx]; !exists {
			return nil, fmt.Errorf("输入特征密文 %d 不存在", vecIdx)
		}
	}

	// 初始化权重梯度矩阵：[hiddenSize][numInputBlocks]
	weightGradients := make([][]*rlwe.Ciphertext, hwge.HiddenSize)
	for i := range weightGradients {
		weightGradients[i] = make([]*rlwe.Ciphertext, hwge.NumInputBlocks)
	}

	// 对每个隐藏神经元计算权重梯度
	for neuronIdx := 0; neuronIdx < hwge.HiddenSize; neuronIdx++ {
		fmt.Printf("\n📊 处理隐藏神经元 %d/%d...\n", neuronIdx+1, hwge.HiddenSize)

		// 步骤1：扩展神经元梯度到整个向量
		expandedGradient, err := hwge.expandNeuronGradient(
			hiddenGradients[neuronIdx], evaluator, encoder, params)
		if err != nil {
			return nil, fmt.Errorf("扩展神经元%d梯度失败: %v", neuronIdx, err)
		}

		// 对每个输入特征块计算权重梯度
		for blockIdx := 0; blockIdx < hwge.NumInputBlocks; blockIdx++ {
			vecIdx := uint64(startVecIdx + blockIdx)

			fmt.Printf("  处理特征块 %d/%d (全局索引: %d)...\n",
				blockIdx+1, hwge.NumInputBlocks, vecIdx)

			// 步骤2：梯度与特征相乘
			gradientProduct, err := evaluator.MulNew(expandedGradient, inputFeatures[vecIdx])
			if err != nil {
				return nil, fmt.Errorf("计算梯度乘积失败(神经元%d, 块%d): %v", neuronIdx, blockIdx, err)
			}

			// 重线性化和Rescale
			if err := evaluator.Relinearize(gradientProduct, gradientProduct); err != nil {
				return nil, fmt.Errorf("重线性化失败(神经元%d, 块%d): %v", neuronIdx, blockIdx, err)
			}
			if err := evaluator.Rescale(gradientProduct, gradientProduct); err != nil {
				return nil, fmt.Errorf("Rescale失败(神经元%d, 块%d): %v", neuronIdx, blockIdx, err)
			}

			// 步骤3：段内聚合求和
			aggregatedGradient, err := hwge.performSegmentAggregation(gradientProduct, evaluator)
			if err != nil {
				return nil, fmt.Errorf("段内聚合失败(神经元%d, 块%d): %v", neuronIdx, blockIdx, err)
			}

			// 步骤4：除以批大小得到最终权重梯度
			batchSizeInv := 1.0 / float64(hwge.BatchSize)
			batchSizeVec := make([]complex128, hwge.Slots)
			for i := 0; i < hwge.Slots; i++ {
				batchSizeVec[i] = complex(batchSizeInv, 0)
			}

			batchSizePt := ckks.NewPlaintext(params, aggregatedGradient.Level())
			if err := encoder.Encode(batchSizeVec, batchSizePt); err != nil {
				return nil, fmt.Errorf("编码批大小失败(神经元%d, 块%d): %v", neuronIdx, blockIdx, err)
			}

			weightGradient, err := evaluator.MulNew(aggregatedGradient, batchSizePt)
			if err != nil {
				return nil, fmt.Errorf("计算最终权重梯度失败(神经元%d, 块%d): %v", neuronIdx, blockIdx, err)
			}
			if err := evaluator.Rescale(weightGradient, weightGradient); err != nil {
				return nil, fmt.Errorf("最终Rescale失败(神经元%d, 块%d): %v", neuronIdx, blockIdx, err)
			}

			weightGradients[neuronIdx][blockIdx] = weightGradient

			fmt.Printf("    ✅ 神经元%d块%d权重梯度计算完成，层级: %d/%d\n",
				neuronIdx, blockIdx, weightGradient.Level(), params.MaxLevel())
		}

		fmt.Printf("  ✅ 神经元%d所有权重梯度计算完成\n", neuronIdx)
	}

	computeTime := time.Since(startTime)
	fmt.Printf("\n🎉 隐藏层权重梯度计算完成！\n")
	fmt.Printf("• 计算时间: %v\n", computeTime)
	fmt.Printf("• 权重梯度矩阵: %d×%d\n", hwge.HiddenSize, hwge.NumInputBlocks)
	fmt.Printf("• 总权重梯度数: %d个密文\n", hwge.HiddenSize*hwge.NumInputBlocks)
	fmt.Printf("• 平均每个权重梯度用时: %.2fms\n",
		float64(computeTime.Nanoseconds())/float64(hwge.HiddenSize*hwge.NumInputBlocks)/1e6)

	return weightGradients, nil
}

// expandNeuronGradient 扩展神经元梯度到整个向量
// 输入：[δ_0^i, δ_1^i, ..., δ_{batchSize-1}^i, 0, 0, ..., 0]
// 输出：[δ_0^i, δ_1^i, ..., δ_{batchSize-1}^i | δ_0^i, δ_1^i, ..., δ_{batchSize-1}^i | ...]
func (hwge *HiddenWeightGradientEngine) expandNeuronGradient(
	neuronGradient *rlwe.Ciphertext,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	params ckks.Parameters,
) (*rlwe.Ciphertext, error) {

	// 计算需要复制的段数
	numSegments := hwge.Slots / hwge.BatchSize

	// 创建扩展后的结果
	result := neuronGradient.CopyNew()

	// 使用旋转和掩码操作将第一段复制到所有段
	for segmentIdx := 1; segmentIdx < numSegments; segmentIdx++ {
		// 计算旋转距离
		rotationDistance := segmentIdx * hwge.BatchSize

		// 旋转得到第一段在新位置的副本
		rotated, err := evaluator.RotateNew(neuronGradient, rotationDistance)
		if err != nil {
			return nil, fmt.Errorf("旋转失败(段%d): %v", segmentIdx, err)
		}

		// 加到结果中
		if err := evaluator.Add(result, rotated, result); err != nil {
			return nil, fmt.Errorf("加法失败(段%d): %v", segmentIdx, err)
		}
	}

	return result, nil
}

// performSegmentAggregation 执行段内聚合求和
// 使用对数级聚合算法将每个段内的样本求和
func (hwge *HiddenWeightGradientEngine) performSegmentAggregation(
	ciphertext *rlwe.Ciphertext,
	evaluator *ckks.Evaluator,
) (*rlwe.Ciphertext, error) {

	result := ciphertext.CopyNew()

	// 对数级聚合：log₂(batchSize)次旋转
	steps := int(math.Log2(float64(hwge.BatchSize)))

	for i := 0; i < steps; i++ {
		step := 1 << i // 2^i

		// 在段内旋转
		rotated, err := evaluator.RotateNew(result, step)
		if err != nil {
			return nil, fmt.Errorf("段内旋转失败(步长%d): %v", step, err)
		}

		// 累加
		if err := evaluator.Add(result, rotated, result); err != nil {
			return nil, fmt.Errorf("段内累加失败(步长%d): %v", step, err)
		}
	}

	return result, nil
}

// VerifyHiddenWeightGradients 验证隐藏层权重梯度的正确性
func (hwge *HiddenWeightGradientEngine) VerifyHiddenWeightGradients(
	weightGradients [][]*rlwe.Ciphertext,
	hiddenGradients []*rlwe.Ciphertext,
	inputFeatures map[uint64]*rlwe.Ciphertext,
	batchIdx int,
	decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder,
) {
	fmt.Println("\n=== 🔍 验证隐藏层权重梯度 ===")

	// 解密隐藏层梯度
	decryptedHiddenGradients := make([][]float64, hwge.HiddenSize)
	for neuronIdx := 0; neuronIdx < hwge.HiddenSize; neuronIdx++ {
		pt := decryptor.DecryptNew(hiddenGradients[neuronIdx])
		decoded := make([]complex128, hwge.Slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密隐藏梯度失败(神经元%d): %v\n", neuronIdx, err)
			continue
		}

		decryptedHiddenGradients[neuronIdx] = make([]float64, hwge.BatchSize)
		for i := 0; i < hwge.BatchSize; i++ {
			decryptedHiddenGradients[neuronIdx][i] = real(decoded[i])
		}
	}

	// 解密输入特征
	startVecIdx := batchIdx * hwge.NumInputBlocks
	decryptedInputFeatures := make([][]float64, hwge.NumInputBlocks)
	for blockIdx := 0; blockIdx < hwge.NumInputBlocks; blockIdx++ {
		vecIdx := uint64(startVecIdx + blockIdx)
		if ct, exists := inputFeatures[vecIdx]; exists {
			pt := decryptor.DecryptNew(ct)
			decoded := make([]complex128, hwge.Slots)
			if err := encoder.Decode(pt, decoded); err != nil {
				fmt.Printf("解密输入特征失败(块%d): %v\n", blockIdx, err)
				continue
			}

			decryptedInputFeatures[blockIdx] = make([]float64, hwge.Slots)
			for i := 0; i < hwge.Slots; i++ {
				decryptedInputFeatures[blockIdx][i] = real(decoded[i])
			}
		}
	}

	// 解密权重梯度
	decryptedWeightGradients := make([][][]float64, hwge.HiddenSize)
	for neuronIdx := 0; neuronIdx < hwge.HiddenSize; neuronIdx++ {
		decryptedWeightGradients[neuronIdx] = make([][]float64, hwge.NumInputBlocks)
		for blockIdx := 0; blockIdx < hwge.NumInputBlocks; blockIdx++ {
			if weightGradients[neuronIdx][blockIdx] != nil {
				pt := decryptor.DecryptNew(weightGradients[neuronIdx][blockIdx])
				decoded := make([]complex128, hwge.Slots)
				if err := encoder.Decode(pt, decoded); err != nil {
					fmt.Printf("解密权重梯度失败(神经元%d,块%d): %v\n", neuronIdx, blockIdx, err)
					continue
				}

				decryptedWeightGradients[neuronIdx][blockIdx] = make([]float64, hwge.K)
				for featIdx := 0; featIdx < hwge.K && featIdx < len(decoded); featIdx++ {
					// 取每个段的第一个值（聚合后所有值相同）
					decryptedWeightGradients[neuronIdx][blockIdx][featIdx] = real(decoded[featIdx*hwge.BatchSize])
				}
			}
		}
	}

	// 计算明文权重梯度进行对比
	fmt.Println("\n📊 权重梯度精度对比（前2个神经元，前2个特征块）：")
	fmt.Println("神经元 | 块 | 特征 | 明文梯度 | 密文梯度 | 绝对误差 | 相对误差")
	fmt.Println("-------|----|----- |----------|----------|----------|----------")

	totalError := 0.0
	maxError := 0.0
	errorCount := 0

	for neuronIdx := 0; neuronIdx < min(2, hwge.HiddenSize); neuronIdx++ {
		for blockIdx := 0; blockIdx < min(2, hwge.NumInputBlocks); blockIdx++ {
			for featIdx := 0; featIdx < min(3, hwge.K); featIdx++ {
				// 计算明文权重梯度：∇W_{neuronIdx, blockIdx*K+featIdx} = (1/batchSize) × Σ δ_m × X_m[f]
				plaintextGradientSum := 0.0
				for sampleIdx := 0; sampleIdx < hwge.BatchSize; sampleIdx++ {
					if neuronIdx < len(decryptedHiddenGradients) &&
						sampleIdx < len(decryptedHiddenGradients[neuronIdx]) &&
						blockIdx < len(decryptedInputFeatures) {

						gradient := decryptedHiddenGradients[neuronIdx][sampleIdx]
						featurePos := featIdx*hwge.BatchSize + sampleIdx
						if featurePos < len(decryptedInputFeatures[blockIdx]) {
							feature := decryptedInputFeatures[blockIdx][featurePos]
							plaintextGradientSum += gradient * feature
						}
					}
				}
				plaintextGradient := plaintextGradientSum / float64(hwge.BatchSize)

				// 获取密文权重梯度
				var ciphertextGradient float64
				if neuronIdx < len(decryptedWeightGradients) &&
					blockIdx < len(decryptedWeightGradients[neuronIdx]) &&
					featIdx < len(decryptedWeightGradients[neuronIdx][blockIdx]) {
					ciphertextGradient = decryptedWeightGradients[neuronIdx][blockIdx][featIdx]
				}

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

				globalFeatIdx := blockIdx*hwge.K + featIdx
				fmt.Printf("%6d | %2d | %4d | %8.6f | %8.6f | %8.3e | %7.3f%%\n",
					neuronIdx, blockIdx, globalFeatIdx, plaintextGradient, ciphertextGradient, absError, relError)
			}
		}
	}

	// 统计信息
	if errorCount > 0 {
		avgError := totalError / float64(errorCount)
		fmt.Printf("\n=== 精度统计 ===\n")
		fmt.Printf("• 平均绝对误差: %.8f\n", avgError)
		fmt.Printf("• 最大绝对误差: %.8f\n", maxError)
		fmt.Printf("• 验证样本数: %d\n", errorCount)

		// 评估精度
		if maxError < 1e-5 {
			fmt.Printf("✅ 隐藏层权重梯度验证通过！(误差 < 1e-5)\n")
		} else if maxError < 1e-3 {
			fmt.Printf("⚠️ 隐藏层权重梯度精度可接受 (1e-5 < 误差 < 1e-3)\n")
		} else {
			fmt.Printf("❌ 隐藏层权重梯度精度不足 (误差 > 1e-3)\n")
		}
	}

	fmt.Printf("\n📈 权重梯度特性分析:\n")
	fmt.Printf("• 权重梯度矩阵大小: %d×%d\n", hwge.HiddenSize, hwge.NumInputBlocks)
	fmt.Printf("• 每个块包含特征数: %d\n", hwge.K)
	fmt.Printf("• 总权重数: %d\n", hwge.HiddenSize*hwge.InputSize)
	fmt.Printf("• 数据格式: 神经元格式（每个神经元独立计算）\n")
}
