package homomorphic

import (
	"MNIST-CNN/pkg/packCNN"
	"MNIST-CNN/pkg/training"
	"MNIST-CNN/pkg/vector_processor"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// FCLayer 同态全连接层
type FCLayer struct {
	InputSize    int
	HiddenSize   int
	Weights      [][]float64
	Biases       []float64 // 添加偏置项
	Assembler    *vector_processor.VectorAssembler
	Activation   ActivationFunction
	UseBootstrap bool
}

// NewFCLayer 创建新的全连接层
func NewFCLayer(inputSize, hiddenSize int, assembler *vector_processor.VectorAssembler) *FCLayer {
	return &FCLayer{
		InputSize:  inputSize,
		HiddenSize: hiddenSize,
		Weights:    packCNN.CreateWeightMatrix(hiddenSize, inputSize),
		Biases:     packCNN.CreateBiasVector(hiddenSize),
		Assembler:  assembler,
		Activation: &IdentityActivation{}, // 默认使用恒等激活函数
	}
}

// NewFCLayerWithActivation 创建带激活函数的全连接层
func NewFCLayerWithActivation(inputSize, hiddenSize int, assembler *vector_processor.VectorAssembler, activation ActivationFunction) *FCLayer {
	return &FCLayer{
		InputSize:  inputSize,
		HiddenSize: hiddenSize,
		Weights:    packCNN.CreateWeightMatrix(hiddenSize, inputSize),
		Biases:     packCNN.CreateBiasVector(hiddenSize),
		Assembler:  assembler,
		Activation: activation,
	}
}

// NewFCLayerWithInitMethod 创建带自定义权重初始化方法的全连接层
// initMethod: 1=Xavier, 2=He, 3=标准正态, 4=均匀分布, 5=小方差正态
func NewFCLayerWithInitMethod(inputSize, hiddenSize int, assembler *vector_processor.VectorAssembler,
	activation ActivationFunction, initMethod int) *FCLayer {
	return &FCLayer{
		InputSize:  inputSize,
		HiddenSize: hiddenSize,
		Weights:    packCNN.CreateWeightMatrixWithMethod(hiddenSize, inputSize, initMethod),
		Biases:     packCNN.CreateBiasVector(hiddenSize),
		Assembler:  assembler,
		Activation: activation,
	}
}

// ComputeEncrypted 执行同态计算
func (fc *FCLayer) ComputeEncrypted(encryptedVectors map[uint64]*rlwe.Ciphertext,
	batchIdx int, params ckks.Parameters, evaluator *ckks.Evaluator,
	encoder *ckks.Encoder, slots int) ([]*rlwe.Ciphertext, error) {

	// 参数
	s := fc.Assembler.S
	k := fc.Assembler.K
	n := fc.Assembler.ImagesPerVector

	// 该批次对应的密文索引范围
	startVecIdx := batchIdx * fc.Assembler.NumVectors

	// 存储每个神经元的输出
	neuronOutputs := make([]*rlwe.Ciphertext, fc.HiddenSize)

	// 对每个输出神经元进行计算
	startTime := time.Now()
	fmt.Println("\n开始计算每个神经元的输出...")

	for neuronIdx := 0; neuronIdx < fc.HiddenSize; neuronIdx++ {
		fmt.Printf("\n计算神经元 %d:\n", neuronIdx)

		var accumulated *rlwe.Ciphertext

		// 处理每轮（每个向量）
		for round := 0; round < fc.Assembler.NumVectors; round++ {
			vecIdx := uint64(startVecIdx + round)

			// 获取当前轮的密文
			ct, exists := encryptedVectors[vecIdx]
			if !exists {
				fmt.Printf("  警告：密文 %d 不存在\n", vecIdx)
				continue
			}

			// 计算本轮处理的特征范围
			featureStart := round * k
			featureEnd := min(featureStart+k, fc.InputSize)
			actualK := featureEnd - featureStart

			if round < 3 || round == fc.Assembler.NumVectors-1 {
				fmt.Printf("  轮 %d: 处理特征 [%d-%d)\n", round, featureStart, featureEnd)
			}

			// 打包权重
			packedWeights := packCNN.PackWeightsForNeuron(fc.Weights[neuronIdx], featureStart, actualK, n, s)

			// 编码权重
			weightPt := ckks.NewPlaintext(params, params.MaxLevel())
			if err := encoder.Encode(packedWeights, weightPt); err != nil {
				log.Printf("编码权重失败: %v", err)
				continue
			}

			// 密文与权重相乘
			mulResult, err := evaluator.MulNew(ct, weightPt)
			if err != nil {
				log.Printf("密文乘法失败: %v", err)
				continue
			}

			// 乘法后立即rescale
			if err := evaluator.Rescale(mulResult, mulResult); err != nil {
				log.Printf("Rescale失败: %v", err)
				continue
			}

			// 累加到结果
			if accumulated == nil {
				accumulated = mulResult
			} else {
				accumulated, err = evaluator.AddNew(accumulated, mulResult)
				if err != nil {
					log.Printf("密文加法失败: %v", err)
					continue
				}
			}
		}

		// 执行旋转累加，将k个特征的结果相加
		fmt.Printf("  执行 %d 次旋转累加...\n", int(math.Log2(float64(k))))
		result := accumulated
		for step := 0; step < int(math.Log2(float64(k))); step++ {
			rotateBy := n * (1 << step)
			fmt.Printf("    尝试旋转 step=%d, rotateBy=%d\n", step, rotateBy)
			rotated, err := evaluator.RotateNew(result, rotateBy)
			if err != nil {
				fmt.Printf("旋转失败 (step=%d, rotate=%d): %v", step, rotateBy, err)
				continue
			}
			fmt.Printf("    ✓ 旋转成功\n")
			result, err = evaluator.AddNew(result, rotated)
			if err != nil {
				fmt.Printf("累加失败: %v", err)
				continue
			}
		}

		// 添加偏置项
		if fc.Biases != nil && neuronIdx < len(fc.Biases) {
			fmt.Printf("  添加偏置项: %.6f\n", fc.Biases[neuronIdx])

			// 打包偏置项
			packedBias := packCNN.PackBiasForNeuron(fc.Biases[neuronIdx], n, s)

			// 编码偏置项
			biasPt := ckks.NewPlaintext(params, params.MaxLevel())
			if err := encoder.Encode(packedBias, biasPt); err != nil {
				log.Printf("编码偏置项失败: %v", err)
			} else {
				// 添加偏置项到结果
				result, err = evaluator.AddNew(result, biasPt)
				if err != nil {
					log.Printf("添加偏置项失败: %v", err)
				}
			}
		}

		// 应用激活函数
		if fc.Activation != nil {
			activatedResult, err := fc.Activation.Apply(result, evaluator, params)
			if err != nil {
				log.Printf("激活函数应用失败: %v", err)
			} else {
				result = activatedResult
			}
		}

		neuronOutputs[neuronIdx] = result
		fmt.Printf("  ✓ 神经元 %d 计算完成（激活函数：%s）\n", neuronIdx, fc.Activation.GetName())
	}

	computeTime := time.Since(startTime)
	fmt.Printf("\n✓ 所有神经元计算完成，用时: %v\n", computeTime)

	return neuronOutputs, nil
}

// VerifyResults 验证计算结果
func (fc *FCLayer) VerifyResults(neuronOutputs []*rlwe.Ciphertext, decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder, plainVectors [][]float32, batchIdx int,
	trainDataset *training.Dataset, slots int) {

	fmt.Println("\n=== 验证计算结果 ===")

	n := fc.Assembler.ImagesPerVector
	startVecIdx := batchIdx * fc.Assembler.NumVectors

	// 验证每个神经元的输出
	for neuronIdx := 0; neuronIdx < min(3, fc.HiddenSize); neuronIdx++ {
		fmt.Printf("\n验证神经元 %d 的输出:\n", neuronIdx)

		if neuronOutputs[neuronIdx] == nil {
			fmt.Println("  错误：输出为空")
			continue
		}

		// 解密
		pt := decryptor.DecryptNew(neuronOutputs[neuronIdx])
		decoded := make([]complex128, slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			log.Printf("解码失败: %v", err)
			continue
		}

		// 计算并对比三种方式的结果
		fmt.Println("\n  三种计算方式对比:")
		fmt.Println("  样本ID | 原始计算 | 重排明文 | 密文计算 | 误差1 | 误差2")
		fmt.Println("  -------|----------|----------|----------|-------|-------")

		for sampleIdx := 0; sampleIdx < min(5, n); sampleIdx++ {
			// 方式1: 原始数据 × 原始权重 + 偏置
			originalResult := fc.Assembler.ComputeOriginalResult(trainDataset, fc.Weights[neuronIdx],
				batchIdx, sampleIdx)

			// 添加偏置项
			if fc.Biases != nil && neuronIdx < len(fc.Biases) {
				originalResult += fc.Biases[neuronIdx]
			}

			// 应用激活函数到原始结果（用于验证）
			activationName := fc.Activation.GetName()
			if strings.Contains(activationName, "sigmoid") || strings.Contains(activationName, "Sigmoid") {
				originalResult = 1 / (math.Exp(-originalResult) + 1)
			}

			// 方式2: 重排数据 × 原始权重 + 偏置（模拟）
			plainResult := fc.Assembler.ComputePlainResult(plainVectors, fc.Weights[neuronIdx],
				startVecIdx, sampleIdx)

			// 添加偏置项
			if fc.Biases != nil && neuronIdx < len(fc.Biases) {
				plainResult += fc.Biases[neuronIdx]
			}

			// 应用激活函数到明文结果
			if strings.Contains(activationName, "sigmoid") || strings.Contains(activationName, "Sigmoid") {
				plainResult = 1 / (math.Exp(-plainResult) + 1)
			}

			// 方式3: 密文计算（重排数据 × 重排权重）
			encryptedResult := real(decoded[sampleIdx])

			// 计算误差
			error1 := math.Abs(originalResult - plainResult)
			error2 := math.Abs(originalResult - encryptedResult)

			fmt.Printf("  %6d | %8.6f | %8.6f | %8.6f | %.3e | %.3e %s\n",
				sampleIdx,
				originalResult,
				plainResult,
				encryptedResult,
				error1,
				error2,
				func() string {
					if error1 < 1e-9 && error2 < 1e-6 {
						return "✓"
					}
					return "⚠"
				}())
		}

		// 额外验证：显示重排权重的计算过程（第一个样本）
		if neuronIdx == 0 {
			fmt.Println("\n  详细计算过程（神经元0，样本0）:")
			fc.Assembler.VerifyPackedComputation(plainVectors, fc.Weights[0], startVecIdx, 0, trainDataset, batchIdx)
		}
	}
}

// PrintStatistics 打印计算统计信息
func (fc *FCLayer) PrintStatistics(computeTime time.Duration, batchLabels [][]byte, batchIdx int) {
	n := fc.Assembler.ImagesPerVector

	// 显示批次标签信息
	if batchIdx < len(batchLabels) {
		fmt.Printf("\n该批次的标签信息:\n")
		fmt.Printf("前10个标签: ")
		for i := 0; i < min(10, len(batchLabels[batchIdx])); i++ {
			fmt.Printf("%d ", batchLabels[batchIdx][i])
		}
		fmt.Println()
	}

	// 统计信息
	fmt.Println("\n=== 计算统计 ===")
	fmt.Printf("• 处理批次大小: %d 个样本\n", n)
	fmt.Printf("• 计算神经元数: %d\n", fc.HiddenSize)
	fmt.Printf("• 激活函数: %s\n", fc.Activation.GetName())
	fmt.Printf("• 总计算时间: %v\n", computeTime)
	fmt.Printf("• 平均每神经元: %v\n", computeTime/time.Duration(fc.HiddenSize))
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ComputeEncryptedWithReorganization 执行同态计算并重组输出
func (fc *FCLayer) ComputeEncryptedWithReorganization(
	encryptedVectors map[uint64]*rlwe.Ciphertext,
	batchIdx int,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	slots int) ([]*rlwe.Ciphertext, error) {

	// 1. 执行原有的计算，得到每个神经元的输出（包含激活函数）
	neuronOutputs, err := fc.ComputeEncrypted(encryptedVectors, batchIdx, params, evaluator, encoder, slots)
	if err != nil {
		return nil, err
	}

	// 2. 创建重组器
	reorganizer := NewMaskReorganizer(fc.Assembler.K, fc.Assembler.ImagesPerVector, slots)
	reorganizer.PrintReorganizationInfo(fc.HiddenSize)

	// 3. 激活函数已在ComputeEncrypted中应用
	fmt.Printf("\n激活函数 %s 已应用\n", fc.Activation.GetName())

	// 4. 重组输出
	fmt.Println("\n开始重组神经元输出...")
	reorganized, err := reorganizer.ReorganizeOutputs(neuronOutputs, params, encoder, evaluator)
	if err != nil {
		return nil, fmt.Errorf("重组失败: %v", err)
	}

	return reorganized, nil
}

// ComputeFromReorganized 从重组后的格式计算下一层
// 输入格式已经是重组后的：[样本0神经元0-k, 样本1神经元0-k, ..., 样本n-1神经元0-k | ...]
func (fc *FCLayer) ComputeFromReorganized(
	reorganizedInputs []*rlwe.Ciphertext,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	slots int) ([]*rlwe.Ciphertext, error) {

	// 参数
	s := fc.Assembler.S
	k := fc.Assembler.K
	n := fc.Assembler.ImagesPerVector

	// 存储每个神经元的输出
	neuronOutputs := make([]*rlwe.Ciphertext, fc.HiddenSize)

	fmt.Println("\n计算下一层...")

	for neuronIdx := 0; neuronIdx < fc.HiddenSize; neuronIdx++ {
		var accumulated *rlwe.Ciphertext

		// 处理每个输入向量
		for vecIdx, inputCt := range reorganizedInputs {
			// 计算这个向量包含的神经元范围
			startNeuron := vecIdx * k
			endNeuron := min(startNeuron+k, fc.InputSize)

			// 打包这个向量对应的权重
			packedWeights := make([]complex128, s)
			for j := 0; j < endNeuron-startNeuron; j++ {
				weight := fc.Weights[neuronIdx][startNeuron+j]
				// 在对应的段中填充权重
				for m := 0; m < n; m++ {
					idx := j*n + m
					if idx < s {
						packedWeights[idx] = complex(weight, 0)
					}
				}
			}

			// 编码权重
			weightPt := ckks.NewPlaintext(params, params.MaxLevel())
			if err := encoder.Encode(packedWeights, weightPt); err != nil {
				return nil, fmt.Errorf("编码权重失败: %v", err)
			}

			// 密文与权重相乘
			mulResult, err := evaluator.MulNew(inputCt, weightPt)
			if err != nil {
				return nil, fmt.Errorf("密文乘法失败: %v", err)
			}

			// 乘法后立即rescale
			if err := evaluator.Rescale(mulResult, mulResult); err != nil {
				return nil, fmt.Errorf("Rescale失败: %v", err)
			}

			// 累加
			if accumulated == nil {
				accumulated = mulResult
			} else {
				accumulated, err = evaluator.AddNew(accumulated, mulResult)
				if err != nil {
					return nil, fmt.Errorf("累加失败: %v", err)
				}
			}
		}

		// 执行旋转累加
		result := accumulated
		for step := 0; step < int(math.Log2(float64(k))); step++ {
			rotateBy := n * (1 << step)
			rotated, err := evaluator.RotateNew(result, rotateBy)
			if err != nil {
				continue
			}
			result, err = evaluator.AddNew(result, rotated)
			if err != nil {
				continue
			}
		}

		// 添加偏置项
		if fc.Biases != nil && neuronIdx < len(fc.Biases) {
			// 打包偏置项
			packedBias := packCNN.PackBiasForNeuron(fc.Biases[neuronIdx], n, s)

			// 编码偏置项
			biasPt := ckks.NewPlaintext(params, params.MaxLevel())
			if err := encoder.Encode(packedBias, biasPt); err != nil {
				log.Printf("编码偏置项失败: %v", err)
			} else {
				// 添加偏置项到结果
				result, err = evaluator.AddNew(result, biasPt)
				if err != nil {
					log.Printf("添加偏置项失败: %v", err)
				}
			}
		}

		// 应用激活函数
		if fc.Activation != nil {
			activatedResult, err := fc.Activation.Apply(result, evaluator, params)
			if err != nil {
				log.Printf("激活函数应用失败: %v", err)
			} else {
				result = activatedResult
			}
		}

		neuronOutputs[neuronIdx] = result
	}

	return neuronOutputs, nil
}

// SetActivation 设置激活函数
func (fc *FCLayer) SetActivation(activation ActivationFunction) {
	fc.Activation = activation
}
