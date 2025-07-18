package packCNN

import (
	"math"
	"math/rand"
)

// PackWeightsForNeuron 为单个神经元打包权重（适用于CKKS）
// neuronWeights: 神经元的权重向量
// start: 起始特征索引
// k: 每个密文处理的特征数
// n: 每个特征段内的样本数
// s: 总槽数
func PackWeightsForNeuron(neuronWeights []float64, start, k, n, s int) []complex128 {
	packed := make([]complex128, s)

	// 按照公式 W_i^{packed}[j·n + m] = W_{i, start+j}
	for j := 0; j < k; j++ {
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
// n: 每个密文处理的样本数
// s: 总槽数
func PackBiasForNeuron(bias float64, n, s int) []complex128 {
	packed := make([]complex128, s)

	// 偏置项在前n个位置重复，其余位置为0
	for i := 0; i < n && i < s; i++ {
		packed[i] = complex(bias, 0)
	}

	return packed
}

// CreateBiasVector 创建并初始化偏置向量
// hiddenSize: 隐藏层神经元数量
func CreateBiasVector(hiddenSize int) []float64 {
	biases := make([]float64, hiddenSize)

	// 偏置项通常初始化为0或小的随机值
	for i := 0; i < hiddenSize; i++ {
		biases[i] = 0.0 // 初始化为0
		// 或者使用小的随机值：biases[i] = (rand.Float64() - 0.5) * 0.01
	}

	return biases
}

// CreateWeightMatrix 创建并初始化权重矩阵
// hiddenSize: 隐藏层神经元数量
// inputSize: 输入特征数量
func CreateWeightMatrix(hiddenSize, inputSize int) [][]float64 {
	return CreateWeightMatrixWithMethod(hiddenSize, inputSize, 1)
}

// CreateWeightMatrixWithMethod 创建并初始化权重矩阵（可选择初始化方法）
// hiddenSize: 隐藏层神经元数量
// inputSize: 输入特征数量
// initMethod: 初始化方法 (1=Xavier, 2=He, 3=标准正态, 4=均匀分布, 5=小方差正态)
func CreateWeightMatrixWithMethod(hiddenSize, inputSize int, initMethod int) [][]float64 {
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
