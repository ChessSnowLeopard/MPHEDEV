package interfaces

import (
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// DataPackingService 数据打包服务接口
type DataPackingService interface {
	GetBatchSize() int
	GetNumVectors() int
	GetSlotCount() int
	GetFeaturesPerCipher() int
	GetParams() ckks.Parameters
	GetEncryptor() *rlwe.Encryptor
	GetEncoder() *ckks.Encoder
	EncryptBatch(images [][]float64) ([]*rlwe.Ciphertext, error)
	GetProcessedImages() int
	AddProcessedImages(count int)

	// 新增：标签重排处理方法
	ProcessLabelsWithPacking(labels []int) ([]*rlwe.Ciphertext, error)
}

// ActivationManager 激活函数管理器接口
type ActivationManager interface {
	// 基本功能
	SetActivationFunction(activationName string)
	SetRole(role string)
	SetSoftmaxProcessor(processor SoftmaxProcessor)
	SetLossFunction(lossFunction LossFunction)
	GetActivationFunction() ActivationFunction
	GetSoftmaxProcessor() SoftmaxProcessor
	GetLossFunction() LossFunction
	PrintConfiguration()

	// 角色判断
	IsInputLayer() bool
	IsHiddenLayer() bool
	IsOutputLayer() bool
	HasSoftmax() bool
	HasLossFunction() bool

	// 刷新服务支持
	SetRefreshService(refreshService interface{}) // 使用interface{}避免循环依赖
	SetOnlinePeers(onlinePeers map[int]string, myID int)
	SetCommonCRSSeed(seed []byte)

	// 带刷新功能的计算
	ApplySoftmaxWithRefresh(
		neuronOutputs []*rlwe.Ciphertext,
		params ckks.Parameters,
		evaluator *ckks.Evaluator,
		encoder *ckks.Encoder,
	) ([]*rlwe.Ciphertext, error)

	ComputeLossWithRefresh(
		softmaxOutputs []*rlwe.Ciphertext,
		labels []int,
		params ckks.Parameters,
		evaluator *ckks.Evaluator,
		encoder *ckks.Encoder,
	) (*rlwe.Ciphertext, error)
}

// DataManager 数据管理器接口
type DataManager interface {
	GetImages() [][]float64
	GetLabels() []int
	GetTotalParticipants() int

	// 批次相关方法
	GetImageBatch(batchID int, batchSize int) [][]float64
	GetLabelBatch(batchID int, batchSize int) []int
	GetTotalBatches(batchSize int) int
	GetBatchSize() int
}

// WeightManager 权重管理器接口
type WeightManager interface {
	// 权重初始化
	CreateWeightMatrix(hiddenSize, inputSize int) [][]float64
	CreateWeightMatrixWithMethod(hiddenSize, inputSize, initMethod int) [][]float64
	CreateBiasVector(hiddenSize int) []float64
	InitializeWeights(hiddenSize, inputSize int, method string) ([][]float64, error)

	// 权重打包
	PackWeightsForNeuron(neuronWeights []float64, start, actualK, n, s int) []complex128
	PackBiasForNeuron(bias float64) []complex128

	// 配置和工具
	PrintConfiguration()
	SetEncoder(encoder *ckks.Encoder)
	GetParams() ckks.Parameters
	GetEncoder() *ckks.Encoder
	GetEncryptor() *rlwe.Encryptor
}

// LayerProcessor 层处理器接口
type LayerProcessor interface {
	ProcessData(images [][]float64) error
}

// FeatureCollector 特征收集器接口（Input Layer专用）
type FeatureCollector interface {
	CollectAllFeatures() error
	OrganizeFeatures() map[int]map[int][]*rlwe.Ciphertext
	GetCollectionStatus() map[int]bool
	IsCollectionComplete() bool
	HandleFeatureData(participantID int, batchID int, features []*rlwe.Ciphertext) error
}

// FeatureSender 特征发送器接口（Hidden/Output Layer专用）
type FeatureSender interface {
	SendFeaturesToInputLayer() error
	GetSentFeatures() map[int][]*rlwe.Ciphertext // [BatchID][]Ciphertext
}

// LabelCollector 标签收集器接口（Output Layer专用）
type LabelCollector interface {
	CollectAllLabels() error
	OrganizeLabels() map[int]map[int][]*rlwe.Ciphertext
	GetLabelCollectionStatus() map[int]bool
	IsLabelCollectionComplete() bool
	HandleLabelData(participantID int, batchID int, labels []*rlwe.Ciphertext) error
}

// LabelSender 标签发送器接口（Input/Hidden Layer专用）
type LabelSender interface {
	SendLabelsToOutputLayer() error
	GetSentLabels() map[int][]*rlwe.Ciphertext // [BatchID][]Ciphertext
}

// ActivationFunction 激活函数接口
type ActivationFunction interface {
	Apply(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error)
	GetName() string
}

// SoftmaxProcessor Softmax处理器接口
type SoftmaxProcessor interface {
	ApplySoftmax(neuronOutputs []*rlwe.Ciphertext, params ckks.Parameters, evaluator *ckks.Evaluator, encoder *ckks.Encoder) ([]*rlwe.Ciphertext, error)
	GetName() string
}

// LossFunction 损失函数接口
type LossFunction interface {
	ComputeLoss(softmaxOutputs []*rlwe.Ciphertext, labels []int, params ckks.Parameters, evaluator *ckks.Evaluator, encoder *ckks.Encoder) (*rlwe.Ciphertext, error)
	GetName() string
}

// Min 辅助函数：返回两个整数中的较小值
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MinSlice 返回切片中的最小值
func MinSlice(values []int) int {
	if len(values) == 0 {
		return 0
	}
	minVal := values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

// MaxSlice 返回切片中的最大值
func MaxSlice(values []int) int {
	if len(values) == 0 {
		return 0
	}
	maxVal := values[0]
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

// MessageSender 消息发送器接口
type MessageSender interface {
	SendMessageToParticipant(targetID int, message string) error
	SendCiphertextData(targetID int, dataType string, batchID int, ciphertexts []*rlwe.Ciphertext) error
}
