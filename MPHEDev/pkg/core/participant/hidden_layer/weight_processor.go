package hidden_layer

import (
	"MPHEDev/pkg/core/participant/interfaces"
	"fmt"
	"math"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// HiddenLayerWeightProcessor 隐藏层权重处理器
type HiddenLayerWeightProcessor struct {
	weightManager      interfaces.WeightManager
	dataPackingService interfaces.DataPackingService

	// 隐藏层配置
	InputSize  int // 输入特征数（来自输入层）
	HiddenSize int // 隐藏层神经元数

	// 权重矩阵（明文）
	Weights [][]float64 // 权重矩阵
	Biases  []float64   // 偏置向量
}

// NewHiddenLayerWeightProcessor 创建隐藏层权重处理器
func NewHiddenLayerWeightProcessor(
	weightManager interfaces.WeightManager,
	dataPackingService interfaces.DataPackingService,
	inputSize, hiddenSize int) *HiddenLayerWeightProcessor {

	return &HiddenLayerWeightProcessor{
		weightManager:      weightManager,
		dataPackingService: dataPackingService,
		InputSize:          inputSize,
		HiddenSize:         hiddenSize,
		Weights:            nil,
		Biases:             nil,
	}
}

// InitializeWeights 初始化权重矩阵
func (hlwp *HiddenLayerWeightProcessor) InitializeWeights(initMethod int) error {
	fmt.Printf("隐藏层：初始化权重矩阵 (%d x %d)...\n", hlwp.HiddenSize, hlwp.InputSize)

	// 创建权重矩阵
	hlwp.Weights = hlwp.weightManager.CreateWeightMatrixWithMethod(hlwp.HiddenSize, hlwp.InputSize, initMethod)

	// 创建偏置向量
	hlwp.Biases = hlwp.weightManager.CreateBiasVector(hlwp.HiddenSize)

	fmt.Printf("✓ 隐藏层权重矩阵初始化完成\n")
	fmt.Printf("  • 权重矩阵: %d x %d\n", len(hlwp.Weights), len(hlwp.Weights[0]))
	fmt.Printf("  • 偏置向量: %d\n", len(hlwp.Biases))

	return nil
}

// ComputeHiddenLayer 执行隐藏层计算
func (hlwp *HiddenLayerWeightProcessor) ComputeHiddenLayer(
	encryptedVectors map[uint64]*rlwe.Ciphertext,
	batchIdx int,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder) ([]*rlwe.Ciphertext, error) {

	if hlwp.Weights == nil {
		return nil, fmt.Errorf("权重矩阵未初始化")
	}

	fmt.Printf("隐藏层：执行前向计算（使用明文权重）...\n")
	fmt.Printf("计算参数:\n")
	fmt.Printf("  • 输入向量数: %d\n", len(encryptedVectors))
	fmt.Printf("  • 批次索引: %d\n", batchIdx)
	fmt.Printf("  • 权重矩阵: %d x %d\n", len(hlwp.Weights), len(hlwp.Weights[0]))
	fmt.Printf("  • 偏置向量: %d\n", len(hlwp.Biases))

	// 获取配置参数
	k := hlwp.dataPackingService.GetSlotCount() / hlwp.dataPackingService.GetBatchSize() // 每图像特征数
	n := hlwp.dataPackingService.GetBatchSize()                                          // 每向量图像数

	// 该批次对应的密文索引范围
	startVecIdx := batchIdx * hlwp.dataPackingService.GetNumVectors()

	// 存储每个神经元的输出
	neuronOutputs := make([]*rlwe.Ciphertext, hlwp.HiddenSize)

	// 对每个输出神经元进行计算
	fmt.Printf("开始计算 %d 个神经元...\n", hlwp.HiddenSize)
	for neuronIdx := 0; neuronIdx < hlwp.HiddenSize; neuronIdx++ {
		fmt.Printf("  计算神经元 %d/%d...\n", neuronIdx+1, hlwp.HiddenSize)

		var accumulated *rlwe.Ciphertext

		// 处理每轮（每个向量）
		for round := 0; round < hlwp.dataPackingService.GetNumVectors(); round++ {
			vecIdx := uint64(startVecIdx + round)

			// 获取当前轮的密文
			ct, exists := encryptedVectors[vecIdx]
			if !exists {
				fmt.Printf("    警告：密文 %d 不存在\n", vecIdx)
				continue
			}

			// 计算本轮处理的特征范围（参考MNIST_CNN项目）
			featureStart := round * k
			featureEnd := min(featureStart+k, hlwp.InputSize)
			actualK := featureEnd - featureStart

			// 打包权重（使用明文权重，传递actualK参数）
			packedWeights := hlwp.weightManager.PackWeightsForNeuron(hlwp.Weights[neuronIdx], featureStart, actualK, n, hlwp.dataPackingService.GetSlotCount())

			// 编码权重（明文编码，不加密）
			weightPt := ckks.NewPlaintext(params, params.MaxLevel())
			if err := encoder.Encode(packedWeights, weightPt); err != nil {
				fmt.Printf("    编码权重失败: %v\n", err)
				continue
			}

			// 密文与权重相乘（密文 × 明文 = 密文）
			mulResult, err := evaluator.MulNew(ct, weightPt)
			if err != nil {
				fmt.Printf("    密文乘法失败: %v\n", err)
				continue
			}

			// 乘法后立即rescale
			if err := evaluator.Rescale(mulResult, mulResult); err != nil {
				fmt.Printf("    Rescale失败: %v\n", err)
				continue
			}

			// 累加到结果
			if accumulated == nil {
				accumulated = mulResult
			} else {
				accumulated, err = evaluator.AddNew(accumulated, mulResult)
				if err != nil {
					fmt.Printf("    密文加法失败: %v\n", err)
					continue
				}
			}
		}

		// 执行旋转累加，将k个特征的结果相加
		result := accumulated
		for step := 0; step < int(math.Log2(float64(k))); step++ {
			rotateBy := n * (1 << step)
			rotated, err := evaluator.RotateNew(result, rotateBy)
			if err != nil {
				fmt.Printf("    旋转失败 (step=%d, rotate=%d): %v\n", step, rotateBy, err)
				continue
			}
			result, err = evaluator.AddNew(result, rotated)
			if err != nil {
				fmt.Printf("    累加失败: %v\n", err)
				continue
			}
		}

		// 添加偏置项
		if hlwp.Biases != nil && neuronIdx < len(hlwp.Biases) {
			// 打包偏置项
			packedBias := hlwp.weightManager.PackBiasForNeuron(hlwp.Biases[neuronIdx])

			// 编码偏置项（明文编码，不加密）
			biasPt := ckks.NewPlaintext(params, params.MaxLevel())
			if err := encoder.Encode(packedBias, biasPt); err != nil {
				fmt.Printf("    编码偏置项失败: %v\n", err)
			} else {
				// 添加偏置项到结果
				result, err = evaluator.AddNew(result, biasPt)
				if err != nil {
					fmt.Printf("    添加偏置项失败: %v\n", err)
				}
			}
		}

		neuronOutputs[neuronIdx] = result
		fmt.Printf("    ✓ 神经元 %d 计算完成\n", neuronIdx+1)
	}

	fmt.Printf("✓ 隐藏层前向计算完成，输出 %d 个神经元\n", len(neuronOutputs))
	return neuronOutputs, nil
}

// GetWeights 获取权重矩阵
func (hlwp *HiddenLayerWeightProcessor) GetWeights() [][]float64 {
	return hlwp.Weights
}

// GetBiases 获取偏置向量
func (hlwp *HiddenLayerWeightProcessor) GetBiases() []float64 {
	return hlwp.Biases
}

// PrintWeightInfo 打印权重信息
func (hlwp *HiddenLayerWeightProcessor) PrintWeightInfo() {
	fmt.Printf("\n=== 隐藏层权重信息 ===\n")
	fmt.Printf("输入特征数: %d\n", hlwp.InputSize)
	fmt.Printf("隐藏层神经元数: %d\n", hlwp.HiddenSize)

	if hlwp.Weights != nil {
		fmt.Printf("权重矩阵: %d x %d\n", len(hlwp.Weights), len(hlwp.Weights[0]))
		fmt.Printf("偏置向量: %d\n", len(hlwp.Biases))
		fmt.Printf("权重类型: 明文权重（用于同态计算）\n")
	} else {
		fmt.Printf("权重矩阵: 未初始化\n")
	}

	fmt.Printf("=====================\n\n")
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
