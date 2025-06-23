package participant

import (
	"fmt"

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
	// 根据激活函数类型和初始化方法两个枚举类设置对应的权重矩阵初始化
	//这里仅作测试示例 后期需要重新写权重初始化
	weights := mat.NewDense(outputSize, inputSize, nil)
	for i := 0; i < outputSize; i++ {
		for j := 0; j < inputSize; j++ {
			// 简单的权重初始化策略（实际应用中可能需要更复杂的初始化方法）
			weights.Set(i, j, 0.1*(float64(i+j)/float64(inputSize+outputSize)))
		}
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
