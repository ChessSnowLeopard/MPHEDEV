package services

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// WeightManager 权重管理器
type WeightManager struct {
	// 配置参数
	SlotCount    int // CKKS slot数
	FeatureCount int // 每图像特征数
	BatchSize    int // 批次大小

	// 加密相关
	Params    ckks.Parameters
	Encoder   *ckks.Encoder
	Encryptor *rlwe.Encryptor
}

// NewWeightManager 创建新的权重管理器
func NewWeightManager(slotCount, featureCount int, params ckks.Parameters, encryptor *rlwe.Encryptor) *WeightManager {
	batchSize := slotCount / featureCount

	return &WeightManager{
		SlotCount:    slotCount,
		FeatureCount: featureCount,
		BatchSize:    batchSize,
		Params:       params,
		Encryptor:    encryptor,
	}
}

// ==================== 权重初始化方法 ====================

// CreateWeightMatrix 创建并初始化权重矩阵
func (wm *WeightManager) CreateWeightMatrix(hiddenSize, inputSize int) [][]float64 {
	return wm.CreateWeightMatrixWithMethod(hiddenSize, inputSize, 2) // 默认使用He初始化
}

// CreateWeightMatrixWithMethod 创建并初始化权重矩阵（可选择初始化方法）
// initMethod: 1=Xavier, 2=He, 3=标准正态, 4=均匀分布, 5=小方差正态
func (wm *WeightManager) CreateWeightMatrixWithMethod(hiddenSize, inputSize int, initMethod int) [][]float64 {
	W := make([][]float64, hiddenSize)

	switch initMethod {
	case 1: // Xavier/Glorot初始化 (适用于Sigmoid/Tanh)
		var fanIn, fanOut float64 = float64(inputSize), float64(hiddenSize)
		var stddev float64 = math.Sqrt(2.0 / (fanIn + fanOut))
		for i := 0; i < hiddenSize; i++ {
			W[i] = make([]float64, inputSize)
			for j := 0; j < inputSize; j++ {
				W[i][j] = rand.NormFloat64() * stddev
			}
		}

	case 2: // He初始化 (适用于ReLU)
		var fanIn float64 = float64(inputSize)
		var stddev float64 = math.Sqrt(2.0 / fanIn)
		for i := 0; i < hiddenSize; i++ {
			W[i] = make([]float64, inputSize)
			for j := 0; j < inputSize; j++ {
				W[i][j] = rand.NormFloat64() * stddev
			}
		}

	case 3: // 标准正态分布 (均值0，方差1)
		for i := 0; i < hiddenSize; i++ {
			W[i] = make([]float64, inputSize)
			for j := 0; j < inputSize; j++ {
				W[i][j] = rand.NormFloat64()
			}
		}

	case 4: // 均匀分布初始化
		var fanIn, fanOut float64 = float64(inputSize), float64(hiddenSize)
		var limit float64 = math.Sqrt(6.0 / (fanIn + fanOut))
		for i := 0; i < hiddenSize; i++ {
			W[i] = make([]float64, inputSize)
			for j := 0; j < inputSize; j++ {
				// 均匀分布 [-limit, limit]
				W[i][j] = (rand.Float64()*2.0 - 1.0) * limit
			}
		}

	case 5: // 较小方差的正态分布
		var stddev float64 = 0.01 // 很小的标准差
		for i := 0; i < hiddenSize; i++ {
			W[i] = make([]float64, inputSize)
			for j := 0; j < inputSize; j++ {
				W[i][j] = rand.NormFloat64() * stddev
			}
		}

	default: // Xavier初始化
		var fanIn, fanOut float64 = float64(inputSize), float64(hiddenSize)
		var stddev float64 = math.Sqrt(2.0 / (fanIn + fanOut))
		for i := 0; i < hiddenSize; i++ {
			W[i] = make([]float64, inputSize)
			for j := 0; j < inputSize; j++ {
				W[i][j] = rand.NormFloat64() * stddev
			}
		}
	}

	return W
}

// CreateBiasVector 创建并初始化偏置向量
func (wm *WeightManager) CreateBiasVector(hiddenSize int) []float64 {
	biases := make([]float64, hiddenSize)

	// 偏置项通常初始化为0或小的随机值
	for i := 0; i < hiddenSize; i++ {
		biases[i] = 0.0 // 初始化为0
		// 或者使用小的随机值：biases[i] = (rand.Float64() - 0.5) * 0.01
	}

	return biases
}

// InitializeWeights 根据方法名初始化权重矩阵
func (wm *WeightManager) InitializeWeights(hiddenSize, inputSize int, method string) ([][]float64, error) {
	var initMethod int

	switch method {
	case "xavier", "Xavier", "glorot", "Glorot":
		initMethod = 1
	case "he", "He":
		initMethod = 2
	case "normal", "Normal":
		initMethod = 3
	case "uniform", "Uniform":
		initMethod = 4
	case "small", "Small":
		initMethod = 5
	default:
		initMethod = 2 // 默认使用He初始化
	}

	weights := wm.CreateWeightMatrixWithMethod(hiddenSize, inputSize, initMethod)
	return weights, nil
}

// ==================== 权重打包方法 ====================

// PackWeightsForNeuron 为单个神经元打包权重（适用于CKKS）
// neuronWeights: 神经元的权重向量
// start: 起始特征索引
// actualK: 实际处理的特征数（可能小于FeatureCount）
// n: 每个特征段内的样本数
// s: 总槽数
func (wm *WeightManager) PackWeightsForNeuron(neuronWeights []float64, start, actualK, n, s int) []complex128 {
	packed := make([]complex128, s)

	// 按照公式 W_i^{packed}[j·n + m] = W_{i, start+j}
	for j := 0; j < actualK; j++ {
		if start+j < len(neuronWeights) {
			weight := neuronWeights[start+j]
			// 每个权重复制n次
			for m := 0; m < n; m++ {
				idx := j*n + m
				if idx < s {
					packed[idx] = complex(weight, 0)
				}
			}
		}
	}

	return packed
}

// PackBiasForNeuron 为单个神经元打包偏置项
// bias: 神经元的偏置值
func (wm *WeightManager) PackBiasForNeuron(bias float64) []complex128 {
	packed := make([]complex128, wm.SlotCount)

	// 偏置项在前n个位置重复，其余位置为0
	for i := 0; i < wm.BatchSize && i < wm.SlotCount; i++ {
		packed[i] = complex(bias, 0)
	}

	return packed
}

// ==================== 工具方法 ====================

// PrintConfiguration 打印配置信息
func (wm *WeightManager) PrintConfiguration() {
	fmt.Printf("\n=== 权重管理器配置 ===\n")
	fmt.Printf("CKKS slot数: %d\n", wm.SlotCount)
	fmt.Printf("每图像特征数: %d\n", wm.FeatureCount)
	fmt.Printf("批次大小: %d\n", wm.BatchSize)
	fmt.Printf("权重打包格式: 神经元 × 特征段\n")
	fmt.Printf("偏置打包格式: 前%d个位置重复\n", wm.BatchSize)
	fmt.Printf("权重处理方式: 明文编码（用于同态计算）\n")
	fmt.Printf("========================\n\n")
}

// SetEncoder 设置编码器
func (wm *WeightManager) SetEncoder(encoder *ckks.Encoder) {
	wm.Encoder = encoder
}

// GetParams 获取CKKS参数
func (wm *WeightManager) GetParams() ckks.Parameters {
	return wm.Params
}

// GetEncoder 获取编码器
func (wm *WeightManager) GetEncoder() *ckks.Encoder {
	return wm.Encoder
}

// GetEncryptor 获取加密器
func (wm *WeightManager) GetEncryptor() *rlwe.Encryptor {
	return wm.Encryptor
}
