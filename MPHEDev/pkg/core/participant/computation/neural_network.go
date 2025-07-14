package computation

import (
	"MPHEDev/pkg/core/participant/interfaces"
	"fmt"
	"math"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// NeuralNetwork 神经网络计算引擎
type NeuralNetwork struct {
	layers             []*Layer
	weightManager      interfaces.WeightManager
	activationManager  interfaces.ActivationManager
	dataPackingService interfaces.DataPackingService
	reorganizer        *MaskReorganizer
}

// Layer 神经网络层
type Layer struct {
	InputSize  int
	HiddenSize int
	Weights    [][]float64
	Biases     []float64
	Activation interfaces.ActivationFunction
	LayerType  string // "input", "hidden", "output"
}

// NewNeuralNetwork 创建神经网络计算引擎
func NewNeuralNetwork(
	weightManager interfaces.WeightManager,
	activationManager interfaces.ActivationManager,
	dataPackingService interfaces.DataPackingService) *NeuralNetwork {

	// 创建重组器
	reorganizer := NewMaskReorganizer(
		dataPackingService.GetFeaturesPerCipher(),
		dataPackingService.GetBatchSize(),
		dataPackingService.GetSlotCount(),
	)

	return &NeuralNetwork{
		weightManager:      weightManager,
		activationManager:  activationManager,
		dataPackingService: dataPackingService,
		reorganizer:        reorganizer,
		layers:             make([]*Layer, 0),
	}
}

// AddLayer 添加层
func (nn *NeuralNetwork) AddLayer(inputSize, hiddenSize int, layerType string) {
	layer := &Layer{
		InputSize:  inputSize,
		HiddenSize: hiddenSize,
		LayerType:  layerType,
	}

	// 根据层类型设置激活函数
	switch layerType {
	case "input":
		layer.Activation = nn.activationManager.GetActivationFunction()
	case "hidden":
		layer.Activation = nn.activationManager.GetActivationFunction()
	case "output":
		layer.Activation = nn.activationManager.GetActivationFunction()
	}

	nn.layers = append(nn.layers, layer)
}

// ForwardPropagation 执行前向传播
func (nn *NeuralNetwork) ForwardPropagation(
	encryptedVectors map[uint64]*rlwe.Ciphertext,
	batchIdx int,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	slots int) ([]*rlwe.Ciphertext, error) {

	fmt.Println("\n=== 开始神经网络前向传播 ===")
	startTime := time.Now()

	var currentOutputs []*rlwe.Ciphertext

	// 逐层计算
	for i, layer := range nn.layers {
		fmt.Printf("\n处理第%d层 (%s层)...\n", i+1, layer.LayerType)
		fmt.Printf("激活函数: %s\n", layer.Activation.GetName())

		var layerOutputs []*rlwe.Ciphertext
		var err error

		if i == 0 {
			// 第一层：从原始打包格式计算
			layerOutputs, err = nn.computeFirstLayer(layer, encryptedVectors, batchIdx, params, evaluator, encoder, slots)
		} else if i == len(nn.layers)-1 {
			// 最后一层：输出层计算
			layerOutputs, err = nn.computeOutputLayer(layer, currentOutputs, params, evaluator, encoder, slots)
		} else {
			// 中间层：从重组格式计算
			layerOutputs, err = nn.computeHiddenLayer(layer, currentOutputs, params, evaluator, encoder, slots)
		}

		if err != nil {
			return nil, fmt.Errorf("第%d层计算失败: %v", i+1, err)
		}

		currentOutputs = layerOutputs
	}

	duration := time.Since(startTime)
	fmt.Printf("\n✓ 前向传播完成，总用时: %v\n", duration)
	fmt.Printf("• 层数: %d\n", len(nn.layers))
	fmt.Printf("• 平均每层用时: %v\n", duration/time.Duration(len(nn.layers)))

	return currentOutputs, nil
}

// computeFirstLayer 计算第一层（输入层）
func (nn *NeuralNetwork) computeFirstLayer(
	layer *Layer,
	encryptedVectors map[uint64]*rlwe.Ciphertext,
	batchIdx int,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	slots int) ([]*rlwe.Ciphertext, error) {

	// 输入层通常不进行权重计算，直接返回重组后的数据
	fmt.Println("输入层：执行数据重组...")

	// 将map转换为slice格式
	var encryptedVectorsSlice []*rlwe.Ciphertext
	for i := uint64(0); i < uint64(len(encryptedVectors)); i++ {
		if ct, exists := encryptedVectors[i]; exists {
			encryptedVectorsSlice = append(encryptedVectorsSlice, ct)
		}
	}

	// 使用已实现的MaskReorganizer进行数据重组
	reorganizedVectors, err := nn.reorganizer.ReorganizeOutputs(encryptedVectorsSlice, params, encoder, evaluator)
	if err != nil {
		return nil, fmt.Errorf("输入层数据重组失败: %v", err)
	}

	fmt.Printf("✓ 输入层数据重组完成，生成 %d 个重组向量\n", len(reorganizedVectors))
	return reorganizedVectors, nil
}

// computeHiddenLayer 计算隐藏层
func (nn *NeuralNetwork) computeHiddenLayer(
	layer *Layer,
	reorganizedInputs []*rlwe.Ciphertext,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	slots int) ([]*rlwe.Ciphertext, error) {

	fmt.Println("隐藏层：执行权重计算和激活函数...")

	// 执行同态计算
	neuronOutputs, err := nn.computeEncrypted(layer, reorganizedInputs, params, evaluator, encoder, slots)
	if err != nil {
		return nil, err
	}

	// 重组输出
	reorganized, err := nn.reorganizeOutputs(neuronOutputs, params, encoder, evaluator)
	if err != nil {
		return nil, fmt.Errorf("隐藏层重组失败: %v", err)
	}

	return reorganized, nil
}

// computeOutputLayer 计算输出层
func (nn *NeuralNetwork) computeOutputLayer(
	layer *Layer,
	reorganizedInputs []*rlwe.Ciphertext,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	slots int) ([]*rlwe.Ciphertext, error) {

	fmt.Println("输出层：执行最终计算...")

	// 执行同态计算（不需要重组）
	neuronOutputs, err := nn.computeEncrypted(layer, reorganizedInputs, params, evaluator, encoder, slots)
	if err != nil {
		return nil, err
	}

	return neuronOutputs, nil
}

// computeEncrypted 执行同态计算（核心计算逻辑）
func (nn *NeuralNetwork) computeEncrypted(
	layer *Layer,
	encryptedVectors []*rlwe.Ciphertext,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	slots int) ([]*rlwe.Ciphertext, error) {

	// 获取参数
	s := nn.dataPackingService.GetSlotCount()
	k := nn.dataPackingService.GetFeaturesPerCipher()
	n := nn.dataPackingService.GetBatchSize()

	// 存储每个神经元的输出
	neuronOutputs := make([]*rlwe.Ciphertext, layer.HiddenSize)

	// 对每个输出神经元进行计算
	startTime := time.Now()
	fmt.Println("\n开始计算每个神经元的输出...")

	for neuronIdx := 0; neuronIdx < layer.HiddenSize; neuronIdx++ {
		fmt.Printf("\n计算神经元 %d:\n", neuronIdx)

		var accumulated *rlwe.Ciphertext

		// 处理每个密文向量
		for vecIdx, ct := range encryptedVectors {
			fmt.Printf("  处理向量 %d...\n", vecIdx)

			// 获取权重
			weights := layer.Weights[neuronIdx]
			if weights == nil {
				return nil, fmt.Errorf("神经元 %d 的权重未初始化", neuronIdx)
			}

			// 打包权重
			packedWeights := nn.packWeightsForNeuron(weights, vecIdx*k, k, n, s)

			// 编码权重
			weightPt := ckks.NewPlaintext(params, params.MaxLevel())
			if err := encoder.Encode(packedWeights, weightPt); err != nil {
				return nil, fmt.Errorf("编码权重失败: %v", err)
			}

			// 密文与权重相乘
			mulResult, err := evaluator.MulNew(ct, weightPt)
			if err != nil {
				return nil, fmt.Errorf("密文乘法失败: %v", err)
			}

			// 乘法后立即rescale
			if err := evaluator.Rescale(mulResult, mulResult); err != nil {
				return nil, fmt.Errorf("Rescale失败: %v", err)
			}

			// 累加到结果
			if accumulated == nil {
				accumulated = mulResult
			} else {
				accumulated, err = evaluator.AddNew(accumulated, mulResult)
				if err != nil {
					return nil, fmt.Errorf("密文加法失败: %v", err)
				}
			}
		}

		// 执行旋转累加，将k个特征的结果相加
		fmt.Printf("  执行 %d 次旋转累加...\n", int(math.Log2(float64(k))))
		result := accumulated
		for step := 0; step < int(math.Log2(float64(k))); step++ {
			rotateBy := n * (1 << step)
			rotated, err := evaluator.RotateNew(result, rotateBy)
			if err != nil {
				return nil, fmt.Errorf("旋转失败 (step=%d, rotate=%d): %v", step, rotateBy, err)
			}
			result, err = evaluator.AddNew(result, rotated)
			if err != nil {
				return nil, fmt.Errorf("累加失败: %v", err)
			}
		}

		// 添加偏置项
		if layer.Biases != nil && neuronIdx < len(layer.Biases) {
			fmt.Printf("  添加偏置项: %.6f\n", layer.Biases[neuronIdx])
			packedBias := nn.packBiasForNeuron(layer.Biases[neuronIdx], n, s)
			biasPt := ckks.NewPlaintext(params, params.MaxLevel())
			if err := encoder.Encode(packedBias, biasPt); err != nil {
				return nil, fmt.Errorf("编码偏置项失败: %v", err)
			}
			_, err := evaluator.AddNew(result, biasPt)
			if err != nil {
				return nil, fmt.Errorf("添加偏置项失败: %v", err)
			}
		}

		// 应用激活函数
		if layer.Activation != nil {
			activatedResult, err := layer.Activation.Apply(result, evaluator, params)
			if err != nil {
				return nil, fmt.Errorf("激活函数应用失败: %v", err)
			}
			result = activatedResult
		}

		neuronOutputs[neuronIdx] = result
		fmt.Printf("  ✓ 神经元 %d 计算完成（激活函数：%s）\n", neuronIdx, layer.Activation.GetName())
	}

	computeTime := time.Since(startTime)
	fmt.Printf("\n✓ 所有神经元计算完成，用时: %v\n", computeTime)

	return neuronOutputs, nil
}

// packWeightsForNeuron 为神经元打包权重
func (nn *NeuralNetwork) packWeightsForNeuron(weights []float64, featureStart, k, n, s int) []complex128 {
	packed := make([]complex128, s)

	for i := 0; i < k && featureStart+i < len(weights); i++ {
		for j := 0; j < n; j++ {
			idx := j*k + i
			if idx < s {
				packed[idx] = complex(weights[featureStart+i], 0)
			}
		}
	}

	return packed
}

// packBiasForNeuron 为神经元打包偏置项
func (nn *NeuralNetwork) packBiasForNeuron(bias float64, n, s int) []complex128 {
	packed := make([]complex128, s)

	for j := 0; j < n; j++ {
		if j < s {
			packed[j] = complex(bias, 0)
		}
	}

	return packed
}

// reorganizeOutputs 重组输出
func (nn *NeuralNetwork) reorganizeOutputs(
	neuronOutputs []*rlwe.Ciphertext,
	params ckks.Parameters,
	encoder *ckks.Encoder,
	evaluator *ckks.Evaluator) ([]*rlwe.Ciphertext, error) {

	fmt.Println("重组输出...")
	return nn.reorganizer.ReorganizeOutputs(neuronOutputs, params, encoder, evaluator)
}

// PrintArchitecture 打印网络架构
func (nn *NeuralNetwork) PrintArchitecture() {
	fmt.Println("\n=== 神经网络架构 ===")

	for i, layer := range nn.layers {
		biasInfo := ""
		if layer.Biases != nil {
			biasInfo = ", 带偏置项"
		}

		if i < len(nn.layers)-1 {
			fmt.Printf("隐藏层%d: %d 个神经元, 激活函数: %s%s\n",
				i+1, layer.HiddenSize, layer.Activation.GetName(), biasInfo)
		} else {
			fmt.Printf("输出层: %d 个神经元, 激活函数: %s%s\n",
				layer.HiddenSize, layer.Activation.GetName(), biasInfo)
		}
	}

	fmt.Printf("\n批处理配置:\n")
	fmt.Printf("• 批大小(n): %d\n", nn.dataPackingService.GetBatchSize())
	fmt.Printf("• 每密文特征数(k): %d\n", nn.dataPackingService.GetFeaturesPerCipher())
	fmt.Printf("• 向量大小(s): %d\n", nn.dataPackingService.GetSlotCount())
	fmt.Println("================")
}
