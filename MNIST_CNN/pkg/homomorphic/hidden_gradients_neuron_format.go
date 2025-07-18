package homomorphic

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// HiddenGradientEngineNeuronFormat 基于神经元格式的隐藏层梯度计算引擎
// 严格按照用户的原始算法，但适配当前系统的神经元格式数据
type HiddenGradientEngineNeuronFormat struct {
	HiddenSize int // 隐藏层神经元数 (t')
	OutputSize int // 输出层神经元数 (t'')
	BatchSize  int // 批大小 (s/k)
	Slots      int // 密文槽数 (s)
	K          int // 每密文特征数
}

// NewHiddenGradientEngineNeuronFormat 创建基于神经元格式的隐藏层梯度计算引擎
func NewHiddenGradientEngineNeuronFormat(hiddenSize, outputSize, batchSize, slots, k int) *HiddenGradientEngineNeuronFormat {
	return &HiddenGradientEngineNeuronFormat{
		HiddenSize: hiddenSize,
		OutputSize: outputSize,
		BatchSize:  batchSize,
		Slots:      slots,
		K:          k,
	}
}

// ComputeHiddenLayerGradients 计算隐藏层梯度（神经元格式版本）
// 根据原始算法公式：δ_{hidden,m}^l = φ'(Z_m^l) × Σ_{j=0}^{t”-1} δ_m^j × W_{j,l}
func (hge *HiddenGradientEngineNeuronFormat) ComputeHiddenLayerGradients(
	outputGradients []*rlwe.Ciphertext, // 输出层梯度：t''个密文，每个输出神经元一个
	hiddenInputs []*rlwe.Ciphertext, // 隐藏层激活前输入：t'个密文，每个隐藏神经元一个
	outputWeights [][]float64, // 输出层权重：[t''][t']矩阵
	activation ActivationFunction, // 隐藏层激活函数
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
) ([]*rlwe.Ciphertext, error) {

	fmt.Println("\n=== 🧠 计算隐藏层梯度（原始算法+神经元格式）===")
	fmt.Printf("算法特点: 严格按照用户原始算法，适配神经元格式数据\n")
	fmt.Printf("隐藏层神经元数: %d, 输出层神经元数: %d, 批大小: %d\n",
		hge.HiddenSize, hge.OutputSize, hge.BatchSize)

	startTime := time.Now()

	// 输入验证
	if len(outputGradients) != hge.OutputSize {
		return nil, fmt.Errorf("输出梯度数量 (%d) 与输出层大小 (%d) 不匹配",
			len(outputGradients), hge.OutputSize)
	}

	if len(hiddenInputs) != hge.HiddenSize {
		return nil, fmt.Errorf("隐藏层输入数量 (%d) 与隐藏层大小 (%d) 不匹配",
			len(hiddenInputs), hge.HiddenSize)
	}

	if len(outputWeights) != hge.OutputSize {
		return nil, fmt.Errorf("权重矩阵行数 (%d) 与输出层大小 (%d) 不匹配",
			len(outputWeights), hge.OutputSize)
	}

	if len(outputWeights[0]) != hge.HiddenSize {
		return nil, fmt.Errorf("权重矩阵列数 (%d) 与隐藏层大小 (%d) 不匹配",
			len(outputWeights[0]), hge.HiddenSize)
	}

	// 存储隐藏层梯度结果
	hiddenGradients := make([]*rlwe.Ciphertext, hge.HiddenSize)

	// 对每个隐藏层神经元计算梯度
	for hiddenIdx := 0; hiddenIdx < hge.HiddenSize; hiddenIdx++ {
		fmt.Printf("\n📊 处理隐藏神经元 %d/%d...\n", hiddenIdx+1, hge.HiddenSize)

		// 步骤1&2：计算后向传播梯度
		// 根据公式：Σ_{j=0}^{t''-1} δ_m^j × W_{j,l}
		backwardGradient, err := hge.computeBackwardGradientForNeuron(
			outputGradients, outputWeights, hiddenIdx, evaluator, encoder, params)
		if err != nil {
			return nil, fmt.Errorf("计算神经元%d后向梯度失败: %v", hiddenIdx, err)
		}

		// 步骤3：乘以激活函数导数
		// φ'(Z_m^l) × Σ_{j=0}^{t''-1} δ_m^j × W_{j,l}
		finalGradient, err := hge.applyActivationDerivativeForNeuron(
			backwardGradient, hiddenInputs[hiddenIdx], activation, evaluator, params)
		if err != nil {
			return nil, fmt.Errorf("计算神经元%d激活函数导数失败: %v", hiddenIdx, err)
		}

		hiddenGradients[hiddenIdx] = finalGradient

		fmt.Printf("  ✅ 神经元%d梯度计算完成，层级: %d/%d\n",
			hiddenIdx, finalGradient.Level(), params.MaxLevel())
	}

	computeTime := time.Since(startTime)
	fmt.Printf("\n🎉 隐藏层梯度计算完成！\n")
	fmt.Printf("• 计算时间: %v\n", computeTime)
	fmt.Printf("• 输出格式: 神经元格式，%d个密文\n", hge.HiddenSize)
	fmt.Printf("• 每个神经元格式: [δ_{hidden,0}^i,δ_{hidden,1}^i...δ_{hidden,s/k-1}^i | 0...0]\n")
	fmt.Printf("• 算法特点: 严格按照原始算法公式，无格式转换\n")

	return hiddenGradients, nil
}

// computeBackwardGradientForNeuron 计算单个隐藏神经元的后向传播梯度
// 实现：Σ_{j=0}^{t”-1} δ_m^j × W_{j,l}
func (hge *HiddenGradientEngineNeuronFormat) computeBackwardGradientForNeuron(
	outputGradients []*rlwe.Ciphertext,
	outputWeights [][]float64,
	hiddenIdx int,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	params ckks.Parameters,
) (*rlwe.Ciphertext, error) {

	fmt.Printf("    步骤1&2：计算后向传播梯度...\n")

	var aggregatedGradient *rlwe.Ciphertext

	// 聚合所有输出神经元的贡献
	for outputIdx := 0; outputIdx < hge.OutputSize; outputIdx++ {
		// 获取权重 W_{outputIdx,hiddenIdx}
		weight := outputWeights[outputIdx][hiddenIdx]

		// 创建权重明文向量：每个权重重复BatchSize次，然后填充0
		weights := make([]complex128, hge.Slots)
		for i := 0; i < hge.BatchSize && i < hge.Slots; i++ {
			weights[i] = complex(weight, 0)
		}
		// 剩余位置保持为0

		weightPt := ckks.NewPlaintext(params, params.MaxLevel())
		if err := encoder.Encode(weights, weightPt); err != nil {
			return nil, fmt.Errorf("编码权重明文失败(输出神经元%d): %v", outputIdx, err)
		}

		// 计算 R_j = δ^j × W_{j,hiddenIdx}  (密文 × 明文)
		R_j, err := evaluator.MulNew(outputGradients[outputIdx], weightPt)
		if err != nil {
			return nil, fmt.Errorf("计算加权梯度失败(输出神经元%d): %v", outputIdx, err)
		}

		// Rescale
		if err := evaluator.Rescale(R_j, R_j); err != nil {
			return nil, fmt.Errorf("Rescale失败(输出神经元%d): %v", outputIdx, err)
		}

		// 累加到聚合梯度
		if aggregatedGradient == nil {
			aggregatedGradient = R_j
		} else {
			if err := evaluator.Add(aggregatedGradient, R_j, aggregatedGradient); err != nil {
				return nil, fmt.Errorf("累加梯度失败(输出神经元%d): %v", outputIdx, err)
			}
		}
	}

	if aggregatedGradient == nil {
		return nil, fmt.Errorf("未能计算任何后向梯度")
	}

	fmt.Printf("      ✅ 后向传播梯度聚合完成，层级: %d/%d\n",
		aggregatedGradient.Level(), params.MaxLevel())

	return aggregatedGradient, nil
}

// applyActivationDerivativeForNeuron 对单个神经元应用激活函数导数
// 实现：φ'(Z_m^l) × backwardGradient
func (hge *HiddenGradientEngineNeuronFormat) applyActivationDerivativeForNeuron(
	backwardGradient *rlwe.Ciphertext,
	hiddenInput *rlwe.Ciphertext, // Z_m^l
	activation ActivationFunction,
	evaluator *ckks.Evaluator,
	params ckks.Parameters,
) (*rlwe.Ciphertext, error) {

	fmt.Printf("    步骤3：乘以激活函数导数...\n")

	// 计算激活函数导数 φ'(Z_m^l)
	activationDerivative, err := activation.ApplyDerivative(hiddenInput, evaluator, params)
	if err != nil {
		return nil, fmt.Errorf("计算激活函数导数失败: %v", err)
	}

	fmt.Printf("      激活函数导数计算完成，层级: %d/%d\n",
		activationDerivative.Level(), params.MaxLevel())

	// 最终隐藏层梯度：φ'(Z_m^l) × backwardGradient  (密文 × 密文)
	finalGradient, err := evaluator.MulNew(activationDerivative, backwardGradient)
	if err != nil {
		return nil, fmt.Errorf("计算最终梯度失败: %v", err)
	}

	// 重线性化和Rescale
	if err := evaluator.Relinearize(finalGradient, finalGradient); err != nil {
		return nil, fmt.Errorf("重线性化失败: %v", err)
	}
	if err := evaluator.Rescale(finalGradient, finalGradient); err != nil {
		return nil, fmt.Errorf("Rescale失败: %v", err)
	}

	fmt.Printf("      ✅ 激活函数导数已应用，层级: %d/%d\n",
		finalGradient.Level(), params.MaxLevel())

	return finalGradient, nil
}

// VerifyHiddenLayerGradients 验证隐藏层梯度的正确性（神经元格式版本）
func (hge *HiddenGradientEngineNeuronFormat) VerifyHiddenLayerGradients(
	hiddenGradients []*rlwe.Ciphertext,
	outputGradients []*rlwe.Ciphertext,
	hiddenInputs []*rlwe.Ciphertext,
	outputWeights [][]float64,
	activation ActivationFunction,
	decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder,
) {
	fmt.Println("\n=== 🔍 验证隐藏层梯度（原始算法+神经元格式）===")

	// 解密隐藏层梯度
	decryptedHiddenGradients := make([][]float64, hge.HiddenSize)
	for neuronIdx := 0; neuronIdx < hge.HiddenSize; neuronIdx++ {
		pt := decryptor.DecryptNew(hiddenGradients[neuronIdx])
		decoded := make([]complex128, hge.Slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密隐藏梯度失败(神经元%d): %v\n", neuronIdx, err)
			continue
		}

		decryptedHiddenGradients[neuronIdx] = make([]float64, hge.BatchSize)
		for i := 0; i < hge.BatchSize && i < len(decoded); i++ {
			decryptedHiddenGradients[neuronIdx][i] = real(decoded[i])
		}
	}

	// 解密输出层梯度
	decryptedOutputGradients := make([][]float64, hge.OutputSize)
	for j := 0; j < hge.OutputSize; j++ {
		pt := decryptor.DecryptNew(outputGradients[j])
		decoded := make([]complex128, hge.Slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密输出梯度失败(神经元%d): %v\n", j, err)
			continue
		}

		decryptedOutputGradients[j] = make([]float64, hge.BatchSize)
		for i := 0; i < hge.BatchSize; i++ {
			decryptedOutputGradients[j][i] = real(decoded[i])
		}
	}

	// 解密隐藏层输入并计算激活函数导数
	decryptedActivationDerivatives := make([][]float64, hge.HiddenSize)
	for neuronIdx := 0; neuronIdx < hge.HiddenSize; neuronIdx++ {
		pt := decryptor.DecryptNew(hiddenInputs[neuronIdx])
		decoded := make([]complex128, hge.Slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密隐藏输入失败(神经元%d): %v\n", neuronIdx, err)
			continue
		}

		decryptedActivationDerivatives[neuronIdx] = make([]float64, hge.BatchSize)
		for i := 0; i < hge.BatchSize && i < len(decoded); i++ {
			input := real(decoded[i])
			derivative := hge.computeTrueActivationDerivative(activation.GetName(), input)
			decryptedActivationDerivatives[neuronIdx][i] = derivative
		}
	}

	// 计算明文隐藏层梯度进行对比（严格按照原始算法公式）
	fmt.Println("\n📊 隐藏层梯度精度对比（前3个神经元，前3个样本）：")
	fmt.Println("算法公式: δ_{hidden,m}^l = φ'(Z_m^l) × Σ_{j=0}^{t''-1} δ_m^j × W_{j,l}")
	fmt.Println("神经元 | 样本 | 明文梯度 | 密文梯度 | 绝对误差 | 相对误差")
	fmt.Println("-------|------|----------|----------|----------|----------")

	totalError := 0.0
	maxError := 0.0
	errorCount := 0

	for neuronIdx := 0; neuronIdx < min(3, hge.HiddenSize); neuronIdx++ {
		for sampleIdx := 0; sampleIdx < min(3, hge.BatchSize); sampleIdx++ {
			// 严格按照原始算法公式计算明文梯度
			// δ_{hidden,m}^l = φ'(Z_m^l) × Σ_{j=0}^{t''-1} δ_m^j × W_{j,l}

			// 步骤1：计算后向传播部分 Σ_{j=0}^{t''-1} δ_m^j × W_{j,l}
			backwardSum := 0.0
			for outputIdx := 0; outputIdx < hge.OutputSize; outputIdx++ {
				if outputIdx < len(decryptedOutputGradients) &&
					sampleIdx < len(decryptedOutputGradients[outputIdx]) {
					delta_output := decryptedOutputGradients[outputIdx][sampleIdx]
					weight := outputWeights[outputIdx][neuronIdx]
					backwardSum += delta_output * weight
				}
			}

			// 步骤2：乘以激活函数导数 φ'(Z_m^l)
			var activationDeriv float64
			if neuronIdx < len(decryptedActivationDerivatives) &&
				sampleIdx < len(decryptedActivationDerivatives[neuronIdx]) {
				activationDeriv = decryptedActivationDerivatives[neuronIdx][sampleIdx]
			}

			// 最终梯度
			plaintextGradient := activationDeriv * backwardSum

			// 获取密文梯度
			var ciphertextGradient float64
			if neuronIdx < len(decryptedHiddenGradients) &&
				sampleIdx < len(decryptedHiddenGradients[neuronIdx]) {
				ciphertextGradient = decryptedHiddenGradients[neuronIdx][sampleIdx]
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

			fmt.Printf("%6d | %4d | %8.6f | %8.6f | %8.3e | %7.3f%%\n",
				neuronIdx, sampleIdx, plaintextGradient, ciphertextGradient, absError, relError)
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
			fmt.Printf("✅ 隐藏层梯度验证通过！(误差 < 1e-5)\n")
		} else if maxError < 1e-3 {
			fmt.Printf("⚠️ 隐藏层梯度精度可接受 (1e-5 < 误差 < 1e-3)\n")
		} else {
			fmt.Printf("❌ 隐藏层梯度精度不足 (误差 > 1e-3)\n")
		}
	}

	fmt.Printf("\n📈 原始算法梯度特性分析:\n")
	fmt.Printf("• 算法特点: 严格按照原始算法公式（神经元格式适配）\n")
	fmt.Printf("• 公式验证: δ_{hidden,m}^l = φ'(Z_m^l) × Σ_{j=0}^{t''-1} δ_m^j × W_{j,l}\n")
	fmt.Printf("• 数据格式: 神经元格式（每个神经元独立处理）\n")
	fmt.Printf("• 神经元数: %d，样本数: %d\n", hge.HiddenSize, hge.BatchSize)
}

// computeTrueActivationDerivative 计算真实的激活函数导数（明文版本）
func (hge *HiddenGradientEngineNeuronFormat) computeTrueActivationDerivative(activationName string, input float64) float64 {
	if strings.Contains(activationName, "Sigmoid") || strings.Contains(activationName, "sigmoid") {
		// sigmoid导数: σ'(x) = σ(x) * (1 - σ(x))
		sigmoid := 1.0 / (1.0 + math.Exp(-input))
		return sigmoid * (1.0 - sigmoid)
	} else if strings.Contains(activationName, "Tanh") || strings.Contains(activationName, "tanh") {
		// tanh导数: tanh'(x) = 1 - tanh²(x)
		tanh := math.Tanh(input)
		return 1.0 - tanh*tanh
	} else if strings.Contains(activationName, "ReLU") || strings.Contains(activationName, "relu") {
		// ReLU导数: ReLU'(x) = x > 0 ? 1 : 0
		if input > 0 {
			return 1.0
		}
		return 0.0
	} else {
		// Identity/Linear激活函数导数: 1
		return 1.0
	}
}
