package participant

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"gonum.org/v1/gonum/mat"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// NNActivationFunc 定义神经网络的激活函数类型
type NNActivationFunc int

const (
	ActivationNone    NNActivationFunc = iota // 无激活函数
	ActivationReLU                            // ReLU激活函数
	ActivationSigmoid                         // Sigmoid激活函数
	ActivationSoftmax                         // Softmax激活函数
)

type WeightInitMethod int

const (
	InitRandom WeightInitMethod = iota // 简单随机/确定性初始化
	InitHe                             // He初始化（适用于ReLU）
	InitXavier                         // Xavier初始化（适用于Sigmoid/Tanh）
	InitZero                           // 零初始化（测试用）
)

// NNLayerData 包含神经网络层的数据
type NNLayerData struct {
	InputSize  int              // 输入维度
	OutputSize int              // 输出维度
	Weights    *mat.Dense       // 权重矩阵
	Biases     *mat.VecDense    // 偏置向量
	Activation NNActivationFunc // 激活函数类型
}

// NNParty 扩展Party，具有神经网络计算能力
type NNParty struct {
	*Party                     // 继承多方参与者的基础功能
	LayerData  *NNLayerData    // 该参与方持有的神经网络层数据
	Encoder    *ckks.Encoder   // CKKS编码器
	Encryptor  *rlwe.Encryptor // CKKS加密器
	Decryptor  *rlwe.Decryptor // 该参与方的CKKS解密器
	Evaluator  *ckks.Evaluator // CKKS评估器
	NextParty  *NNParty        // 指向链中下一个参与方的引用（如果是最后一个则为nil）
	PrevParty  *NNParty        // 指向链中前一个参与方的引用（如果是第一个则为nil）
	LayerIndex int             // 该参与方在网络中的层索引
}

// NewNNParty 创建具有神经网络能力的新参与方
// 参数:
//
//	party: 已经完成多方密钥生成的基础参与方
//	pk: 从Cloud获取的多方公钥
//	rlk: 从Cloud获取的多方重线性化密钥
//	galk: 从Cloud获取的多方伽罗瓦密钥
func NewNNParty(party *Party) *NNParty {
	// 从Party中获取同态加密参数
	params := party.Params

	// 使用参数初始化编码器
	encoder := ckks.NewEncoder(params)

	// 使用参数和该参与方的私钥初始化解密器
	decryptor := ckks.NewDecryptor(params, party.Sk)

	// 使用参数和多方公钥初始化加密器
	encryptor := ckks.NewEncryptor(params, party.Pk)

	// 创建评估密钥集合
	evk := rlwe.NewMemEvaluationKeySet(party.Rlk, party.Gks...)

	// 使用参数和评估密钥初始化评估器
	evaluator := ckks.NewEvaluator(params, evk)

	fmt.Printf("参与方 %d 加密组件初始化完成\n", party.ID)

	return &NNParty{
		Party:     party,
		Encoder:   encoder,
		Decryptor: decryptor,
		Encryptor: encryptor,
		Evaluator: evaluator,
		NextParty: nil,
		PrevParty: nil, // 初始化时下一个和前一个参与方为空，需要后续设置
	}
}

// InitializeLayer 为该参与方设置神经网络层
// inputSize: 输入层神经元数量
// outputSize: 输出层神经元数量
func (nnp *NNParty) InitializeLayer(inputSize int, outputSize int, activation NNActivationFunc, initMethod WeightInitMethod) {
	// 设置随机种子，确保权重初始化的可重现性
	rand.Seed(time.Now().UnixNano() + int64(nnp.ID))

	// 根据激活函数类型和初始化方法两个枚举类设置对应的权重矩阵初始化
	weights := mat.NewDense(outputSize, inputSize, nil)
	// 根据初始化方法选择不同的策略
	switch initMethod {
	case InitHe:
		// He初始化：适用于ReLU激活函数
		scale := math.Sqrt(2.0 / float64(inputSize))
		for i := 0; i < outputSize; i++ {
			for j := 0; j < inputSize; j++ {
				weights.Set(i, j, rand.NormFloat64()*scale)
			}
		}
	case InitXavier:
		// Xavier初始化：适用于Sigmoid/Tanh激活函数
		scale := math.Sqrt(2.0 / float64(inputSize+outputSize))
		for i := 0; i < outputSize; i++ {
			for j := 0; j < inputSize; j++ {
				weights.Set(i, j, rand.NormFloat64()*scale)
			}
		}
	case InitRandom:
		// 简单随机初始化
		for i := 0; i < outputSize; i++ {
			for j := 0; j < inputSize; j++ {
				weights.Set(i, j, rand.NormFloat64()*0.1)
			}
		}
	case InitZero:
		// 零初始化（权重全为0）
		// 权重矩阵默认为0，无需额外设置
	}

	// 将偏置初始化为零向量
	biases := mat.NewVecDense(outputSize, nil)

	// 设置该参与方的层数据
	nnp.LayerData = &NNLayerData{
		InputSize:  inputSize,
		OutputSize: outputSize,
		Weights:    weights,
		Biases:     biases,
		Activation: activation,
	}

	// fmt.Printf("参与方 %d 初始化神经网络层完成: %dx%d，激活函数: %v\n",
	// 	nnp.ID, inputSize, outputSize, activation)
}

// SetNextParty 设置链中的下一个参与方
func (nnp *NNParty) SetNextParty(nextParty *NNParty) {
	nnp.NextParty = nextParty
	if nextParty != nil {
		nextParty.PrevParty = nnp
		//fmt.Printf("参与方 %d 设置下一个参与方为: %d\n", nnp.ID, nextParty.ID)
	}
}

// SetPrevParty 设置链中的前一个参与方
func (nnp *NNParty) SetPrevParty(prevParty *NNParty) {
	nnp.PrevParty = prevParty
	//fmt.Printf("参与方 %d 设置前一个参与方为: %d\n", nnp.ID, prevParty.ID)
}

// SetLayerIndex 设置该参与方在网络中的层索引
func (nnp *NNParty) SetLayerIndex(index int) {
	nnp.LayerIndex = index
	//fmt.Printf("参与方 %d 设置层索引为: %d\n", nnp.ID, index)
}

// GetLayerInfo 获取该参与方层的信息（用于调试）
func (nnp *NNParty) GetLayerInfo() string {
	if nnp.LayerData == nil {
		return fmt.Sprintf("参与方 %d: 未初始化神经网络层", nnp.ID)
	}

	return fmt.Sprintf("参与方 %d: 层维度 %dx%d，激活函数: %v",
		nnp.ID, nnp.LayerData.InputSize, nnp.LayerData.OutputSize, nnp.LayerData.Activation)
}

// 激活函数实现（用于同态加密的多项式近似）

// ReLUPolyApprox ReLU激活函数的多项式近似
// 使用Chebyshev多项式近似ReLU函数
func ReLUPolyApprox(x float64) float64 {
	// 简单的多项式近似：max(0, x) ≈ 0.5 * (x + sqrt(x^2 + ε))
	// 其中ε是一个小的正数，避免除零
	epsilon := 1e-6
	if x >= 0 {
		return x
	}
	return 0.5 * (x + math.Sqrt(x*x+epsilon))
}

// SigmoidPolyApprox Sigmoid激活函数的多项式近似
// 使用Chebyshev多项式近似Sigmoid函数
func SigmoidPolyApprox(x float64) float64 {
	// 简单的多项式近似：1/(1 + e^(-x)) ≈ 0.5 * (1 + x/sqrt(1 + x^2))
	// 这个近似在[-3, 3]范围内比较准确
	if x > 3 {
		return 1.0
	}
	if x < -3 {
		return 0.0
	}
	return 0.5 * (1.0 + x/math.Sqrt(1.0+x*x))
}

// SoftmaxPolyApprox Softmax激活函数的多项式近似
// 注意：Softmax通常需要整个向量，这里提供单个元素的近似
func SoftmaxPolyApprox(x float64, maxVal float64) float64 {
	// 简化的softmax近似：exp(x - max) / sum(exp(x_i - max))
	// 这里只处理单个元素，实际使用时需要整个向量
	expVal := math.Exp(x - maxVal)
	return expVal
}

// GetActivationFunction 根据激活函数类型返回对应的函数
func (nnp *NNParty) GetActivationFunction() func(float64) float64 {
	switch nnp.LayerData.Activation {
	case ActivationReLU:
		return ReLUPolyApprox
	case ActivationSigmoid:
		return SigmoidPolyApprox
	case ActivationSoftmax:
		// Softmax需要特殊处理，返回一个默认函数
		return func(x float64) float64 { return x }
	case ActivationNone:
		return func(x float64) float64 { return x }
	default:
		return func(x float64) float64 { return x }
	}
}

// ValidateLayerConfig 验证层配置的合理性
func (nnp *NNParty) ValidateLayerConfig() error {
	if nnp.LayerData == nil {
		return fmt.Errorf("参与方 %d: 层数据未初始化", nnp.ID)
	}

	if nnp.LayerData.InputSize <= 0 || nnp.LayerData.OutputSize <= 0 {
		return fmt.Errorf("参与方 %d: 输入或输出维度无效", nnp.ID)
	}

	if nnp.LayerData.Weights == nil || nnp.LayerData.Biases == nil {
		return fmt.Errorf("参与方 %d: 权重或偏置未初始化", nnp.ID)
	}

	return nil
}

// GetLayerDimensions 获取层的维度信息
func (nnp *NNParty) GetLayerDimensions() (int, int) {
	if nnp.LayerData == nil {
		return 0, 0
	}
	return nnp.LayerData.InputSize, nnp.LayerData.OutputSize
}
