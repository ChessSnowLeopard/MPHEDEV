package types

import (
	"fmt"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// ==================== 系统配置参数 ====================

// SystemConfig 系统配置参数
type SystemConfig struct {
	Slots         int `json:"slots"`          // CKKS槽数
	BatchSize     int `json:"batch_size"`     // 批次大小
	K             int `json:"k"`              // 每图像特征数
	N             int `json:"n"`              // 每向量图像数
	NumVectors    int `json:"num_vectors"`    // 向量总数
	ImageFeatures int `json:"image_features"` // 图像特征总数
	HiddenSize    int `json:"hidden_size"`    // 隐藏层大小
	OutputSize    int `json:"output_size"`    // 输出层大小
}

// ==================== 输入层数据结构 ====================

// InputLayerData 输入层数据
type InputLayerData struct {
	EncryptedVectors map[uint64]*rlwe.Ciphertext `json:"encrypted_vectors"` // 加密后的向量
	BatchIndex       int                         `json:"batch_index"`       // 批次索引
	NumVectors       int                         `json:"num_vectors"`       // 向量数量
	VectorSize       int                         `json:"vector_size"`       // 向量大小
	Metadata         InputLayerMetadata          `json:"metadata"`          // 元数据
}

// InputLayerMetadata 输入层元数据
type InputLayerMetadata struct {
	PackingFormat string `json:"packing_format"`  // 打包格式
	Normalization string `json:"normalization"`   // 归一化方式
	DataSplitType string `json:"data_split_type"` // 数据分割类型
	ParticipantID int    `json:"participant_id"`  // 参与方ID
	Timestamp     int64  `json:"timestamp"`       // 时间戳
}

// ==================== 隐藏层数据结构 ====================

// HiddenLayerData 隐藏层数据
type HiddenLayerData struct {
	ReorganizedVectors []*rlwe.Ciphertext  `json:"reorganized_vectors"` // 重组后的密文向量
	NumNeurons         int                 `json:"num_neurons"`         // 神经元数量
	NumOutputVectors   int                 `json:"num_output_vectors"`  // 输出向量数
	ActivationType     string              `json:"activation_type"`     // 激活函数类型
	Metadata           HiddenLayerMetadata `json:"metadata"`            // 元数据
}

// HiddenLayerMetadata 隐藏层元数据
type HiddenLayerMetadata struct {
	InputSize            int      `json:"input_size"`            // 输入大小
	HiddenSize           int      `json:"hidden_size"`           // 隐藏层大小
	ReorganizationMethod string   `json:"reorganization_method"` // 重组方法
	ComputationSteps     []string `json:"computation_steps"`     // 计算步骤
	ParticipantID        int      `json:"participant_id"`        // 参与方ID
	Timestamp            int64    `json:"timestamp"`             // 时间戳
}

// ==================== 输出层数据结构 ====================

// OutputLayerData 输出层数据
type OutputLayerData struct {
	Logits         []*rlwe.Ciphertext  `json:"logits"`          // 原始logits
	SoftmaxOutputs []*rlwe.Ciphertext  `json:"softmax_outputs"` // softmax后的概率
	Loss           *rlwe.Ciphertext    `json:"loss"`            // 批次平均损失
	NumClasses     int                 `json:"num_classes"`     // 类别数量
	Metadata       OutputLayerMetadata `json:"metadata"`        // 元数据
}

// OutputLayerMetadata 输出层元数据
type OutputLayerMetadata struct {
	InputSize      int    `json:"input_size"`      // 输入大小
	OutputSize     int    `json:"output_size"`     // 输出大小
	LossType       string `json:"loss_type"`       // 损失函数类型
	LabelFormat    string `json:"label_format"`    // 标签格式
	EfficiencyGain string `json:"efficiency_gain"` // 效率提升
	ParticipantID  int    `json:"participant_id"`  // 参与方ID
	Timestamp      int64  `json:"timestamp"`       // 时间戳
}

// ==================== 层间通信消息 ====================

// LayerMessage 层间通信消息
type LayerMessage struct {
	MessageType string          `json:"message_type"` // 消息类型
	SourceLayer string          `json:"source_layer"` // 源层
	TargetLayer string          `json:"target_layer"` // 目标层
	BatchIndex  int             `json:"batch_index"`  // 批次索引
	Data        interface{}     `json:"data"`         // 数据内容
	Metadata    MessageMetadata `json:"metadata"`     // 消息元数据
}

// MessageMetadata 消息元数据
type MessageMetadata struct {
	MessageID     string `json:"message_id"`     // 消息ID
	SequenceNum   int    `json:"sequence_num"`   // 序列号
	TotalMessages int    `json:"total_messages"` // 总消息数
	Timestamp     int64  `json:"timestamp"`      // 时间戳
	Status        string `json:"status"`         // 状态
}

// ==================== 消息类型常量 ====================

const (
	// 消息类型
	MessageTypeInputToHidden  = "input_to_hidden"
	MessageTypeHiddenToOutput = "hidden_to_output"
	MessageTypeOutputToLoss   = "output_to_loss"
	MessageTypeLossResult     = "loss_result"

	// 层类型
	LayerTypeInput  = "input_layer"
	LayerTypeHidden = "hidden_layer"
	LayerTypeOutput = "output_layer"

	// 状态
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusError      = "error"
)

// ==================== 数据流控制结构 ====================

// DataFlowControl 数据流控制
type DataFlowControl struct {
	CurrentBatch int                 `json:"current_batch"` // 当前批次
	TotalBatches int                 `json:"total_batches"` // 总批次数
	LayerStatus  map[string]string   `json:"layer_status"`  // 各层状态
	BatchStatus  map[int]BatchStatus `json:"batch_status"`  // 批次状态
	ErrorInfo    *ErrorInfo          `json:"error_info"`    // 错误信息
}

// BatchStatus 批次状态
type BatchStatus struct {
	BatchIndex      int   `json:"batch_index"`       // 批次索引
	InputLayerDone  bool  `json:"input_layer_done"`  // 输入层完成
	HiddenLayerDone bool  `json:"hidden_layer_done"` // 隐藏层完成
	OutputLayerDone bool  `json:"output_layer_done"` // 输出层完成
	LossComputed    bool  `json:"loss_computed"`     // 损失计算完成
	StartTime       int64 `json:"start_time"`        // 开始时间
	EndTime         int64 `json:"end_time"`          // 结束时间
	Duration        int64 `json:"duration"`          // 持续时间
}

// ErrorInfo 错误信息
type ErrorInfo struct {
	ErrorCode    string `json:"error_code"`    // 错误代码
	ErrorMessage string `json:"error_message"` // 错误消息
	Layer        string `json:"layer"`         // 发生错误的层
	Timestamp    int64  `json:"timestamp"`     // 时间戳
}

// ==================== 工具函数 ====================

// NewInputLayerData 创建输入层数据
func NewInputLayerData(encryptedVectors map[uint64]*rlwe.Ciphertext, batchIndex int, config SystemConfig) *InputLayerData {
	return &InputLayerData{
		EncryptedVectors: encryptedVectors,
		BatchIndex:       batchIndex,
		NumVectors:       config.NumVectors,
		VectorSize:       config.Slots,
		Metadata: InputLayerMetadata{
			PackingFormat: "interleaved_features",
			Normalization: "divide_by_255",
			Timestamp:     getCurrentTimestamp(),
		},
	}
}

// NewHiddenLayerData 创建隐藏层数据
func NewHiddenLayerData(reorganizedVectors []*rlwe.Ciphertext, config SystemConfig) *HiddenLayerData {
	return &HiddenLayerData{
		ReorganizedVectors: reorganizedVectors,
		NumNeurons:         config.HiddenSize,
		NumOutputVectors:   (config.HiddenSize + config.K - 1) / config.K,
		ActivationType:     "ReLU",
		Metadata: HiddenLayerMetadata{
			InputSize:            config.ImageFeatures,
			HiddenSize:           config.HiddenSize,
			ReorganizationMethod: "mask_based",
			ComputationSteps: []string{
				"1. 权重打包：将权重矩阵按神经元和特征维度打包",
				"2. 密文乘法：输入向量与打包权重相乘",
				"3. 旋转累加：使用log2(k)次旋转实现特征累加",
				"4. 偏置添加：添加偏置项到结果",
				"5. 激活函数：应用ReLU激活函数",
				"6. 输出重组：使用掩码重组输出格式",
			},
			Timestamp: getCurrentTimestamp(),
		},
	}
}

// NewOutputLayerData 创建输出层数据
func NewOutputLayerData(logits []*rlwe.Ciphertext, softmaxOutputs []*rlwe.Ciphertext, loss *rlwe.Ciphertext, config SystemConfig) *OutputLayerData {
	return &OutputLayerData{
		Logits:         logits,
		SoftmaxOutputs: softmaxOutputs,
		Loss:           loss,
		NumClasses:     config.OutputSize,
		Metadata: OutputLayerMetadata{
			InputSize:      config.HiddenSize,
			OutputSize:     config.OutputSize,
			LossType:       "cross_entropy",
			LabelFormat:    "one_hot_encoded",
			EfficiencyGain: "38x_improvement",
			Timestamp:      getCurrentTimestamp(),
		},
	}
}

// NewLayerMessage 创建层间通信消息
func NewLayerMessage(messageType, sourceLayer, targetLayer string, batchIndex int, data interface{}) *LayerMessage {
	return &LayerMessage{
		MessageType: messageType,
		SourceLayer: sourceLayer,
		TargetLayer: targetLayer,
		BatchIndex:  batchIndex,
		Data:        data,
		Metadata: MessageMetadata{
			MessageID:   generateMessageID(),
			SequenceNum: 0,
			Timestamp:   getCurrentTimestamp(),
			Status:      StatusPending,
		},
	}
}

// getCurrentTimestamp 获取当前时间戳
func getCurrentTimestamp() int64 {
	return time.Now().UnixNano()
}

// generateMessageID 生成消息ID
func generateMessageID() string {
	return fmt.Sprintf("msg_%d", getCurrentTimestamp())
}

// ==================== 通用层间通信数据结构 ====================

// LayerCommunicationData 通用的层间通信数据结构
type LayerCommunicationData struct {
	BatchID     int                `json:"batch_id"`    // 批次ID
	SourceID    int                `json:"source_id"`   // 发送方参与方ID
	TargetID    int                `json:"target_id"`   // 接收方参与方ID
	Timestamp   time.Time          `json:"timestamp"`   // 时间戳
	Ciphertexts []CiphertextVector `json:"ciphertexts"` // 加密的向量数据
	Metadata    LayerMetadata      `json:"metadata"`    // 层元数据
}

// CiphertextVector 密文向量结构（可重用）
type CiphertextVector struct {
	VectorID     int    `json:"vector_id"`     // 向量ID
	Ciphertext   string `json:"ciphertext"`    // Base64编码的密文
	FeatureCount int    `json:"feature_count"` // 特征数量
	SlotCount    int    `json:"slot_count"`    // CKKS槽位数
}

// LayerMetadata 通用层元数据（可扩展）
type LayerMetadata struct {
	LayerType    string                 `json:"layer_type"`    // 层类型：input_layer, hidden_layer, output_layer
	DataType     string                 `json:"data_type"`     // 数据类型：features, labels, weights
	BatchSize    int                    `json:"batch_size"`    // 批次大小
	FeatureCount int                    `json:"feature_count"` // 特征数量
	VectorCount  int                    `json:"vector_count"`  // 向量数量
	DataFormat   string                 `json:"data_format"`   // 数据格式标识
	ExtraInfo    map[string]interface{} `json:"extra_info"`    // 额外信息
}

// ==================== 数据类型常量 ====================

const (
	// 数据类型
	DataTypeFeatures  = "features"  // 特征数据
	DataTypeLabels    = "labels"    // 标签数据
	DataTypeWeights   = "weights"   // 权重数据
	DataTypeGradients = "gradients" // 梯度数据

	// 数据格式
	DataFormatInterleaved = "interleaved_features" // 交错特征格式
	DataFormatSequential  = "sequential_features"  // 顺序特征格式
	DataFormatOneHot      = "one_hot_labels"       // One-hot标签格式
)

// ==================== 通用通信工具函数 ====================

// NewLayerCommunicationData 创建新的层间通信数据
func NewLayerCommunicationData(batchID, sourceID, targetID int, layerType, dataType string) *LayerCommunicationData {
	return &LayerCommunicationData{
		BatchID:     batchID,
		SourceID:    sourceID,
		TargetID:    targetID,
		Timestamp:   time.Now(),
		Ciphertexts: make([]CiphertextVector, 0),
		Metadata: LayerMetadata{
			LayerType: layerType,
			DataType:  dataType,
			ExtraInfo: make(map[string]interface{}),
		},
	}
}

// AddCiphertextVector 添加密文向量
func (lcd *LayerCommunicationData) AddCiphertextVector(vectorID int, ciphertext string, featureCount, slotCount int) {
	lcd.Ciphertexts = append(lcd.Ciphertexts, CiphertextVector{
		VectorID:     vectorID,
		Ciphertext:   ciphertext,
		FeatureCount: featureCount,
		SlotCount:    slotCount,
	})
}

// SetMetadataInfo 设置元数据信息
func (lcd *LayerCommunicationData) SetMetadataInfo(batchSize, featureCount, vectorCount int, dataFormat string) {
	lcd.Metadata.BatchSize = batchSize
	lcd.Metadata.FeatureCount = featureCount
	lcd.Metadata.VectorCount = vectorCount
	lcd.Metadata.DataFormat = dataFormat
}

// AddExtraInfo 添加额外信息
func (lcd *LayerCommunicationData) AddExtraInfo(key string, value interface{}) {
	if lcd.Metadata.ExtraInfo == nil {
		lcd.Metadata.ExtraInfo = make(map[string]interface{})
	}
	lcd.Metadata.ExtraInfo[key] = value
}

// ValidateData 验证数据完整性
func (lcd *LayerCommunicationData) ValidateData() error {
	if lcd.BatchID < 0 {
		return fmt.Errorf("无效的批次ID: %d", lcd.BatchID)
	}
	if lcd.SourceID <= 0 || lcd.TargetID <= 0 {
		return fmt.Errorf("无效的参与方ID: source=%d, target=%d", lcd.SourceID, lcd.TargetID)
	}
	if len(lcd.Ciphertexts) == 0 {
		return fmt.Errorf("密文向量列表为空")
	}
	if lcd.Metadata.LayerType == "" || lcd.Metadata.DataType == "" {
		return fmt.Errorf("缺少层类型或数据类型信息")
	}
	return nil
}
