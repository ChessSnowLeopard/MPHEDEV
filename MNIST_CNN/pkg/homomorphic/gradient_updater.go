package homomorphic

import (
	"fmt"
	"math"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// GradientUpdater 梯度更新引擎
// 负责使用计算得到的梯度更新神经网络的权重和偏置
type GradientUpdater struct {
	LearningRate   float64 // 学习率
	BatchSize      int     // 批大小
	Slots          int     // 密文槽数
	K              int     // 每密文特征数
	NumInputBlocks int     // 输入特征块数
}

// NewGradientUpdater 创建梯度更新引擎
func NewGradientUpdater(learningRate float64, batchSize, slots, k, inputSize int) *GradientUpdater {
	numInputBlocks := (inputSize + k - 1) / k // 向上取整
	return &GradientUpdater{
		LearningRate:   learningRate,
		BatchSize:      batchSize,
		Slots:          slots,
		K:              k,
		NumInputBlocks: numInputBlocks,
	}
}

// UpdateHiddenLayerWeights 更新隐藏层权重
// 根据公式：W_new = W_old - learningRate × gradient
func (gu *GradientUpdater) UpdateHiddenLayerWeights(
	hiddenLayer *FCLayer,
	weightGradients [][]*rlwe.Ciphertext, // [hiddenSize][numInputBlocks]
	decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder,
) error {

	fmt.Println("\n=== 🔄 更新隐藏层权重 ===")
	fmt.Printf("学习率: %.6f, 隐藏神经元数: %d, 输入特征块数: %d\n",
		gu.LearningRate, hiddenLayer.HiddenSize, gu.NumInputBlocks)

	startTime := time.Now()

	// 验证输入
	if len(weightGradients) != hiddenLayer.HiddenSize {
		return fmt.Errorf("权重梯度数量 (%d) 与隐藏层大小 (%d) 不匹配",
			len(weightGradients), hiddenLayer.HiddenSize)
	}

	totalUpdates := 0
	maxGradient := 0.0
	sumGradient := 0.0

	// 对每个隐藏神经元更新权重
	for neuronIdx := 0; neuronIdx < hiddenLayer.HiddenSize; neuronIdx++ {
		fmt.Printf("\n📊 更新神经元 %d/%d 的权重...\n", neuronIdx+1, hiddenLayer.HiddenSize)

		if len(weightGradients[neuronIdx]) != gu.NumInputBlocks {
			return fmt.Errorf("神经元%d的梯度块数 (%d) 与期望块数 (%d) 不匹配",
				neuronIdx, len(weightGradients[neuronIdx]), gu.NumInputBlocks)
		}

		// 处理每个特征块的权重
		for blockIdx := 0; blockIdx < gu.NumInputBlocks; blockIdx++ {
			// 解密梯度
			gradientPt := decryptor.DecryptNew(weightGradients[neuronIdx][blockIdx])

			// 解码梯度
			gradientVec := make([]complex128, gu.Slots)
			if err := encoder.Decode(gradientPt, gradientVec); err != nil {
				return fmt.Errorf("解码权重梯度失败(神经元%d, 块%d): %v", neuronIdx, blockIdx, err)
			}

			// 计算该块对应的特征范围
			featureStart := blockIdx * gu.K
			featureEnd := min(featureStart+gu.K, hiddenLayer.InputSize)

			// 更新权重
			for featOffset := 0; featOffset < featureEnd-featureStart; featOffset++ {
				globalFeatIdx := featureStart + featOffset

				// 从梯度向量中提取该特征的梯度（取第一个段的第一个值）
				gradientValue := real(gradientVec[featOffset*gu.BatchSize])

				// 更新权重：W_new = W_old - learningRate × gradient
				oldWeight := hiddenLayer.Weights[neuronIdx][globalFeatIdx]
				newWeight := oldWeight - gu.LearningRate*gradientValue

				hiddenLayer.Weights[neuronIdx][globalFeatIdx] = newWeight

				// 统计信息
				totalUpdates++
				absGradient := math.Abs(gradientValue)
				if absGradient > maxGradient {
					maxGradient = absGradient
				}
				sumGradient += absGradient

				// 详细日志（仅显示前几个）
				if blockIdx < 2 && featOffset < 3 {
					fmt.Printf("  特征 %d: 梯度=%.6e, 权重 %.6f → %.6f\n",
						globalFeatIdx, gradientValue, oldWeight, newWeight)
				}
			}
		}
	}

	avgGradient := sumGradient / float64(totalUpdates)
	elapsed := time.Since(startTime)

	fmt.Printf("\n✅ 隐藏层权重更新完成！\n")
	fmt.Printf("📊 更新统计:\n")
	fmt.Printf("  - 总更新次数: %d\n", totalUpdates)
	fmt.Printf("  - 最大梯度幅度: %.6e\n", maxGradient)
	fmt.Printf("  - 平均梯度幅度: %.6e\n", avgGradient)
	fmt.Printf("  - 用时: %v\n", elapsed)

	return nil
}

// UpdateOutputLayerWeights 更新输出层权重
func (gu *GradientUpdater) UpdateOutputLayerWeights(
	outputLayer *FCLayer,
	weightGradients [][]*rlwe.Ciphertext, // [outputSize][numHiddenBlocks]
	decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder,
) error {

	fmt.Println("\n=== 🔄 更新输出层权重 ===")
	fmt.Printf("学习率: %.6f, 输出神经元数: %d\n",
		gu.LearningRate, outputLayer.HiddenSize)

	startTime := time.Now()

	// 验证输入
	if len(weightGradients) != outputLayer.HiddenSize {
		return fmt.Errorf("权重梯度数量 (%d) 与输出层大小 (%d) 不匹配",
			len(weightGradients), outputLayer.HiddenSize)
	}

	// 计算隐藏层块数（输出层的输入就是隐藏层的输出）
	numHiddenBlocks := (outputLayer.InputSize + gu.K - 1) / gu.K

	totalUpdates := 0
	maxGradient := 0.0
	sumGradient := 0.0

	// 对每个输出神经元更新权重
	for outputIdx := 0; outputIdx < outputLayer.HiddenSize; outputIdx++ {
		fmt.Printf("\n📊 更新输出神经元 %d/%d 的权重...\n", outputIdx+1, outputLayer.HiddenSize)

		if len(weightGradients[outputIdx]) != numHiddenBlocks {
			return fmt.Errorf("输出神经元%d的梯度块数 (%d) 与期望块数 (%d) 不匹配",
				outputIdx, len(weightGradients[outputIdx]), numHiddenBlocks)
		}

		// 处理每个隐藏层特征块的权重
		for blockIdx := 0; blockIdx < numHiddenBlocks; blockIdx++ {
			// 解密梯度
			gradientPt := decryptor.DecryptNew(weightGradients[outputIdx][blockIdx])

			// 解码梯度
			gradientVec := make([]complex128, gu.Slots)
			if err := encoder.Decode(gradientPt, gradientVec); err != nil {
				return fmt.Errorf("解码权重梯度失败(输出神经元%d, 块%d): %v", outputIdx, blockIdx, err)
			}

			// 计算该块对应的隐藏神经元范围
			hiddenStart := blockIdx * gu.K
			hiddenEnd := min(hiddenStart+gu.K, outputLayer.InputSize)

			// 更新权重
			for hiddenOffset := 0; hiddenOffset < hiddenEnd-hiddenStart; hiddenOffset++ {
				globalHiddenIdx := hiddenStart + hiddenOffset

				// 从梯度向量中提取该隐藏神经元的梯度（取第一个段的第一个值）
				gradientValue := real(gradientVec[hiddenOffset*gu.BatchSize])

				// 更新权重：W_new = W_old - learningRate × gradient
				oldWeight := outputLayer.Weights[outputIdx][globalHiddenIdx]
				newWeight := oldWeight - gu.LearningRate*gradientValue

				outputLayer.Weights[outputIdx][globalHiddenIdx] = newWeight

				// 统计信息
				totalUpdates++
				absGradient := math.Abs(gradientValue)
				if absGradient > maxGradient {
					maxGradient = absGradient
				}
				sumGradient += absGradient

				// 详细日志（仅显示前几个）
				if blockIdx < 2 && hiddenOffset < 3 {
					fmt.Printf("  隐藏神经元 %d: 梯度=%.6e, 权重 %.6f → %.6f\n",
						globalHiddenIdx, gradientValue, oldWeight, newWeight)
				}
			}
		}
	}

	avgGradient := sumGradient / float64(totalUpdates)
	elapsed := time.Since(startTime)

	fmt.Printf("\n✅ 输出层权重更新完成！\n")
	fmt.Printf("📊 更新统计:\n")
	fmt.Printf("  - 总更新次数: %d\n", totalUpdates)
	fmt.Printf("  - 最大梯度幅度: %.6e\n", maxGradient)
	fmt.Printf("  - 平均梯度幅度: %.6e\n", avgGradient)
	fmt.Printf("  - 用时: %v\n", elapsed)

	return nil
}

// UpdateBiases 更新偏置项
func (gu *GradientUpdater) UpdateBiases(
	layer *FCLayer,
	biasGradients []*rlwe.Ciphertext, // 每个神经元一个偏置梯度密文
	decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder,
) error {

	fmt.Println("\n=== 🔄 更新偏置项 ===")
	fmt.Printf("学习率: %.6f, 神经元数: %d\n", gu.LearningRate, layer.HiddenSize)

	startTime := time.Now()

	// 验证输入
	if len(biasGradients) != layer.HiddenSize {
		return fmt.Errorf("偏置梯度数量 (%d) 与层大小 (%d) 不匹配",
			len(biasGradients), layer.HiddenSize)
	}

	maxGradient := 0.0
	sumGradient := 0.0

	// 对每个神经元更新偏置
	for neuronIdx := 0; neuronIdx < layer.HiddenSize; neuronIdx++ {
		// 解密偏置梯度
		gradientPt := decryptor.DecryptNew(biasGradients[neuronIdx])

		// 解码偏置梯度
		gradientVec := make([]complex128, gu.Slots)
		if err := encoder.Decode(gradientPt, gradientVec); err != nil {
			return fmt.Errorf("解码偏置梯度失败(神经元%d): %v", neuronIdx, err)
		}

		// 提取偏置梯度值（通常是向量第一个元素）
		gradientValue := real(gradientVec[0])

		// 更新偏置：B_new = B_old - learningRate × gradient
		oldBias := layer.Biases[neuronIdx]
		newBias := oldBias - gu.LearningRate*gradientValue

		layer.Biases[neuronIdx] = newBias

		// 统计信息
		absGradient := math.Abs(gradientValue)
		if absGradient > maxGradient {
			maxGradient = absGradient
		}
		sumGradient += absGradient

		// 详细日志（仅显示前几个）
		if neuronIdx < 5 {
			fmt.Printf("  神经元 %d: 梯度=%.6e, 偏置 %.6f → %.6f\n",
				neuronIdx, gradientValue, oldBias, newBias)
		}
	}

	avgGradient := sumGradient / float64(layer.HiddenSize)
	elapsed := time.Since(startTime)

	fmt.Printf("\n✅ 偏置更新完成！\n")
	fmt.Printf("📊 更新统计:\n")
	fmt.Printf("  - 最大梯度幅度: %.6e\n", maxGradient)
	fmt.Printf("  - 平均梯度幅度: %.6e\n", avgGradient)
	fmt.Printf("  - 用时: %v\n", elapsed)

	return nil
}

// SetLearningRate 设置学习率
func (gu *GradientUpdater) SetLearningRate(lr float64) {
	gu.LearningRate = lr
	fmt.Printf("🎯 学习率更新为: %.6f\n", lr)
}

// GetLearningRate 获取当前学习率
func (gu *GradientUpdater) GetLearningRate() float64 {
	return gu.LearningRate
}
