package network

import (
	"gonum.org/v1/gonum/stat/distuv"
	"math"
)

// DPSGDConfig 差分隐私SGD配置
type DPSGDConfig struct {
	// L2范数裁剪阈值
	L2NormClip float64
	// 噪声乘数
	NoiseMultiplier float64
	// 批次大小
	BatchSize int
	// 学习率
	LearningRate float64
	// 隐私预算目标
	Delta float64
	// 随机数种子
	Seed int64
}

// NewDPSGDConfig 创建一个默认的DPSGD配置
func NewDPSGDConfig() *DPSGDConfig {
	return &DPSGDConfig{
		L2NormClip:      1.0,
		NoiseMultiplier: 1.0,
		BatchSize:       64,
		LearningRate:    0.01,
		Delta:           1e-5,
		Seed:            42,
	}
}

// ClipGradientByL2Norm 通过L2范数裁剪梯度
func ClipGradientByL2Norm(grad *Gradients, maxNorm float64) []float64 {
	// 创建一个切片存储每层的L2范数
	layerNorms := make([]float64, len(grad.WeightGrads))

	// 计算每层梯度的L2范数
	for i := range grad.WeightGrads {
		weightGradNorm := 0.0
		biasGradNorm := 0.0

		// 计算权重梯度的L2范数
		for j := 0; j < grad.WeightGrads[i].RawMatrix().Rows; j++ {
			for k := 0; k < grad.WeightGrads[i].RawMatrix().Cols; k++ {
				weightGradNorm += math.Pow(grad.WeightGrads[i].At(j, k), 2)
			}
		}

		// 计算偏置梯度的L2范数
		for j := 0; j < grad.BiasGrads[i].Len(); j++ {
			biasGradNorm += math.Pow(grad.BiasGrads[i].AtVec(j), 2)
		}

		totalNorm := math.Sqrt(weightGradNorm + biasGradNorm)
		layerNorms[i] = totalNorm

		// 如果范数超过阈值，进行裁剪
		if totalNorm > maxNorm {
			scaleFactor := maxNorm / totalNorm
			// 缩放权重梯度
			for j := 0; j < grad.WeightGrads[i].RawMatrix().Rows; j++ {
				for k := 0; k < grad.WeightGrads[i].RawMatrix().Cols; k++ {
					grad.WeightGrads[i].Set(j, k, grad.WeightGrads[i].At(j, k)*scaleFactor)
				}
			}

			// 缩放偏置梯度
			for j := 0; j < grad.BiasGrads[i].Len(); j++ {
				grad.BiasGrads[i].SetVec(j, grad.BiasGrads[i].AtVec(j)*scaleFactor)
			}

			// 更新范数为裁剪后的值
			layerNorms[i] = maxNorm
		}
	}
	return layerNorms
}

// AddGaussianNoise 添加高斯噪声到梯度
func AddGaussianNoise(grad *Gradients, sigma, l2NormClip float64, seed int64) {
	// 计算噪声标准差
	// sigma 是噪声乘数，l2NormClip是裁剪阈值
	// 标准差 = 噪声乘数 * 裁剪阈值
	stdDev := sigma * l2NormClip

	// 创建高斯分布
	normal := distuv.Normal{
		Mu:    0,
		Sigma: stdDev,
		Src:   nil, // 使用默认随机源
	}

	// 为每一层的梯度添加噪声
	for i := range grad.WeightGrads {
		// 为权重梯度添加噪声
		for j := 0; j < grad.WeightGrads[i].RawMatrix().Rows; j++ {
			for k := 0; k < grad.WeightGrads[i].RawMatrix().Cols; k++ {
				noise := normal.Rand()
				grad.WeightGrads[i].Set(j, k, grad.WeightGrads[i].At(j, k)+noise)
			}
		}

		// 为偏置梯度添加噪声
		for j := 0; j < grad.BiasGrads[i].Len(); j++ {
			noise := normal.Rand()
			grad.BiasGrads[i].SetVec(j, grad.BiasGrads[i].AtVec(j)+noise)
		}
	}
}

/*
之后RDP部分将在python上利用opacus库来实现
*/

//// ComputeRDP 计算RDP隐私损失
//func ComputeRDP(q, noiseMultiplier float64, steps int, orders []float64) []float64 {
//	rdpValues := make([]float64, len(orders))
//
//	for i, alpha := range orders {
//		// 计算单步RDP
//		singleStepRDP := computeSingleStepRDP(q, noiseMultiplier, alpha)
//
//		// 总RDP = 单步RDP * 步数
//		rdpValues[i] = singleStepRDP * float64(steps)
//	}
//
//	return rdpValues
//}
//
//// computeSingleStepRDP 计算单步RDP隐私损失，基于采样高斯机制
//func computeSingleStepRDP(q, sigma float64, alpha float64) float64 {
//	// 如果没有采样或没有添加噪声，就没有隐私保护
//	if q == 0 || sigma == 0 {
//		return 0
//	}
//
//	// 当采样率为1时，相当于没有采样
//	if q == 1.0 {
//		return alpha / (2 * sigma * sigma)
//	}
//
//	// 对于无限阶RDP，直接返回无限大
//	if math.IsInf(alpha, 1) {
//		return math.Inf(1)
//	}
//
//	// 根据alpha是否为整数使用不同的计算方法
//	if alpha == float64(int(alpha)) {
//		return computeRDPForIntAlpha(q, sigma, int(alpha))
//	} else {
//		return computeRDPForFracAlpha(q, sigma, alpha)
//	}
//}
//
//// computeRDPForIntAlpha 计算整数阶RDP
//func computeRDPForIntAlpha(q, sigma float64, alpha int) float64 {
//	rdp := math.Inf(-1) // 初始化为负无穷
//
//	for i := 0; i <= alpha; i++ {
//		// 计算二项式系数
//		binomCoeff := binomialCoefficient(alpha, i)
//
//		// 计算对数项
//		logB := math.Log(binomCoeff) +
//			float64(i)*math.Log(q) +
//			float64(alpha-i)*math.Log(1-q) +
//			float64(i*i-i)/(2*sigma*sigma)
//
//		// 使用log-sum-exp技巧来避免数值溢出
//		rdp = logAddExp(rdp, logB)
//	}
//
//	// 除以(alpha-1)得到RDP
//	return rdp / float64(alpha-1)
//}
//
//// computeRDPForFracAlpha 计算分数阶RDP
//func computeRDPForFracAlpha(q, sigma, alpha float64) float64 {
//	lowerAlpha := math.Floor(alpha)
//	upperAlpha := math.Ceil(alpha)
//
//	if lowerAlpha == upperAlpha {
//		return computeRDPForIntAlpha(q, sigma, int(alpha))
//	}
//
//	lowerRDP := computeRDPForIntAlpha(q, sigma, int(lowerAlpha))
//	upperRDP := computeRDPForIntAlpha(q, sigma, int(upperAlpha))
//
//	// 线性插值
//	fraction := alpha - lowerAlpha
//	return lowerRDP*(1-fraction) + upperRDP*fraction
//}
//
//// ConvertRDPtoDP 将RDP转换为DP
//func ConvertRDPtoDP(orders []float64, rdpValues []float64, delta float64) (float64, float64) {
//	minEpsilon := math.Inf(1)
//	optOrder := 0.0
//
//	for i, alpha := range orders {
//		// 使用转换公式: ε = rdp + log(1/δ)/(α-1)
//		epsilon := rdpValues[i] + math.Log(1/delta)/(alpha-1)
//
//		if epsilon < minEpsilon {
//			minEpsilon = epsilon
//			optOrder = alpha
//		}
//	}
//
//	return minEpsilon, optOrder
//}
//
//// 工具函数
//
//// binomialCoefficient 计算二项式系数 C(n,k)
//func binomialCoefficient(n, k int) float64 {
//	if k < 0 || k > n {
//		return 0
//	}
//	if k == 0 || k == n {
//		return 1
//	}
//
//	// 使用对称性减少计算量
//	if k > n-k {
//		k = n - k
//	}
//
//	c := 1.0
//	for i := 0; i < k; i++ {
//		c = c * float64(n-i) / float64(i+1)
//	}
//	return c
//}
//
//// logAddExp 计算log(exp(a) + exp(b))，避免数值溢出
//func logAddExp(a, b float64) float64 {
//	if a == math.Inf(-1) { // 如果a是负无穷，相当于加0
//		return b
//	}
//	if b == math.Inf(-1) { // 如果b是负无穷，相当于加0
//		return a
//	}
//
//	// 使用log(exp(a) + exp(b)) = max(a,b) + log(1 + exp(min(a,b) - max(a,b)))
//	maxVal := math.Max(a, b)
//	minVal := math.Min(a, b)
//	return maxVal + math.Log1p(math.Exp(minVal-maxVal))
//}
//
//// logSubExp 计算log(exp(a) - exp(b))，避免数值溢出
//func logSubExp(a, b float64) float64 {
//	if a <= b {
//		panic("结果必须为非负数")
//	}
//	if b == math.Inf(-1) { // 减0
//		return a
//	}
//	if a == b {
//		return math.Inf(-1) // log(0)
//	}
//
//	// 使用log(exp(a) - exp(b)) = a + log(1 - exp(b-a))
//	return a + math.Log1p(-math.Exp(b-a))
//}
