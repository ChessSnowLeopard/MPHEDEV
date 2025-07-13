package data_management

import (
	"MPHEDev/pkg/core/participant/types"
)

// DataManager 数据管理器
type DataManager struct {
	// 数据集相关
	Images    [][]float64 // 载入的图像数据
	Labels    []int       // 载入的标签数据
	DataSplit string      // 数据分片类型：vertical 或 horizontal

	// 角色相关
	TotalParticipants int    // 总参与方数量（从协调器获取）
	Role              string // 当前参与方角色：input_layer, hidden_layer, output_layer
	TestMode          bool   // 是否为测试模式（参与方数量<3）

	// 数据分发状态
	ReceivedFeatures     map[int]bool // 已接收特征数据的参与方ID
	ReceivedLabels       map[int]bool // 已接收标签数据的参与方ID
	DataDistributionDone bool         // 数据分发是否完成

	// 输入层和输出层Done状态
	InputLayerDone  bool // 输入层数据分发完成
	OutputLayerDone bool // 输出层数据分发完成

	// 分批接收状态管理
	FeatureBatchStatus map[int]*types.BatchStatus // 每个参与方的特征批次状态
	LabelBatchStatus   map[int]*types.BatchStatus // 每个参与方的标签批次状态

	// 接收到的密文数据存储
	ReceivedFeatureCiphertexts map[int][][]string // 每个参与方的特征密文批次
	ReceivedLabelCiphertexts   map[int][][]string // 每个参与方的标签密文批次
}

// NewDataManager 创建新的数据管理器
func NewDataManager() *DataManager {
	return &DataManager{
		TotalParticipants:          0,
		Role:                       "",
		TestMode:                   false,
		ReceivedFeatures:           make(map[int]bool),
		ReceivedLabels:             make(map[int]bool),
		DataDistributionDone:       false,
		InputLayerDone:             false,
		OutputLayerDone:            false,
		FeatureBatchStatus:         make(map[int]*types.BatchStatus),
		LabelBatchStatus:           make(map[int]*types.BatchStatus),
		ReceivedFeatureCiphertexts: make(map[int][][]string),
		ReceivedLabelCiphertexts:   make(map[int][][]string),
	}
}

// GetImages 获取图像数据
func (dm *DataManager) GetImages() [][]float64 {
	return dm.Images
}

// SetImages 设置图像数据
func (dm *DataManager) SetImages(images [][]float64) {
	dm.Images = images
}

// GetLabels 获取标签数据
func (dm *DataManager) GetLabels() []int {
	return dm.Labels
}

// SetLabels 设置标签数据
func (dm *DataManager) SetLabels(labels []int) {
	dm.Labels = labels
}

// GetDataSplit 获取数据分片类型
func (dm *DataManager) GetDataSplit() string {
	return dm.DataSplit
}

// SetDataSplit 设置数据分片类型
func (dm *DataManager) SetDataSplit(dataSplit string) {
	dm.DataSplit = dataSplit
}

// GetReceivedFeatures 获取已接收特征数据的参与方
func (dm *DataManager) GetReceivedFeatures() map[int]bool {
	return dm.ReceivedFeatures
}

// SetReceivedFeatures 设置已接收特征数据的参与方
func (dm *DataManager) SetReceivedFeatures(receivedFeatures map[int]bool) {
	dm.ReceivedFeatures = receivedFeatures
}

// GetReceivedLabels 获取已接收标签数据的参与方
func (dm *DataManager) GetReceivedLabels() map[int]bool {
	return dm.ReceivedLabels
}

// SetReceivedLabels 设置已接收标签数据的参与方
func (dm *DataManager) SetReceivedLabels(receivedLabels map[int]bool) {
	dm.ReceivedLabels = receivedLabels
}

// IsDataDistributionDone 检查数据分发是否完成
func (dm *DataManager) IsDataDistributionDone() bool {
	return dm.DataDistributionDone
}

// SetDataDistributionDone 设置数据分发完成状态
func (dm *DataManager) SetDataDistributionDone(done bool) {
	dm.DataDistributionDone = done
}

// IsInputLayerDone 检查输入层是否完成
func (dm *DataManager) IsInputLayerDone() bool {
	return dm.InputLayerDone
}

// SetInputLayerDone 设置输入层完成状态
func (dm *DataManager) SetInputLayerDone(done bool) {
	dm.InputLayerDone = done
}

// IsOutputLayerDone 检查输出层是否完成
func (dm *DataManager) IsOutputLayerDone() bool {
	return dm.OutputLayerDone
}

// SetOutputLayerDone 设置输出层完成状态
func (dm *DataManager) SetOutputLayerDone(done bool) {
	dm.OutputLayerDone = done
}

// GetFeatureBatchStatus 获取特征批次状态
func (dm *DataManager) GetFeatureBatchStatus() map[int]*types.BatchStatus {
	return dm.FeatureBatchStatus
}

// GetLabelBatchStatus 获取标签批次状态
func (dm *DataManager) GetLabelBatchStatus() map[int]*types.BatchStatus {
	return dm.LabelBatchStatus
}

// GetReceivedFeatureCiphertexts 获取接收到的特征密文
func (dm *DataManager) GetReceivedFeatureCiphertexts() map[int][][]string {
	return dm.ReceivedFeatureCiphertexts
}

// GetReceivedLabelCiphertexts 获取接收到的标签密文
func (dm *DataManager) GetReceivedLabelCiphertexts() map[int][][]string {
	return dm.ReceivedLabelCiphertexts
}

// ==================== 角色相关方法 ====================

// SetTotalParticipants 设置总参与方数量
func (dm *DataManager) SetTotalParticipants(total int) {
	dm.TotalParticipants = total
}

// GetTotalParticipants 获取总参与方数量
func (dm *DataManager) GetTotalParticipants() int {
	return dm.TotalParticipants
}

// DetermineRole 根据参与方ID和总参与方数量判定角色
func (dm *DataManager) DetermineRole(participantID int) {
	// 首先判断是否为测试模式
	if dm.TotalParticipants < 3 {
		dm.TestMode = true
		// 测试模式下的角色判定
		switch dm.TotalParticipants {
		case 1:
			// 1个参与方：既是输入层又是输出层
			dm.Role = "input_output_layer"
		case 2:
			// 2个参与方：1是输入层，2是输出层
			if participantID == 1 {
				dm.Role = "input_layer"
			} else {
				dm.Role = "output_layer"
			}
		default:
			dm.Role = "unknown"
		}
	} else {
		// 正常模式下的角色判定
		dm.TestMode = false
		if participantID == 1 {
			dm.Role = "input_layer"
		} else if participantID == dm.TotalParticipants {
			dm.Role = "output_layer"
		} else {
			dm.Role = "hidden_layer"
		}
	}
}

// GetRole 获取当前角色
func (dm *DataManager) GetRole() string {
	return dm.Role
}

// IsTestMode 检查是否为测试模式
func (dm *DataManager) IsTestMode() bool {
	return dm.TestMode
}

// IsInputLayer 检查是否为输入层
func (dm *DataManager) IsInputLayer() bool {
	return dm.Role == "input_layer" || dm.Role == "input_output_layer"
}

// IsHiddenLayer 检查是否为隐藏层
func (dm *DataManager) IsHiddenLayer() bool {
	return dm.Role == "hidden_layer"
}

// IsOutputLayer 检查是否为输出层
func (dm *DataManager) IsOutputLayer() bool {
	return dm.Role == "output_layer" || dm.Role == "input_output_layer"
}

// GetRoleDescription 获取角色描述
func (dm *DataManager) GetRoleDescription() string {
	switch dm.Role {
	case "input_layer":
		return "输入层"
	case "hidden_layer":
		return "隐藏层"
	case "output_layer":
		return "输出层"
	case "input_output_layer":
		return "输入输出层（测试模式）"
	default:
		return "未知角色"
	}
}

// ==================== 批次相关方法 ====================

// GetBatchSize 获取默认批次大小
func (dm *DataManager) GetBatchSize() int {
	return 10 // 默认批次大小，可以根据需要调整
}

// GetTotalBatches 计算总批次数
func (dm *DataManager) GetTotalBatches(batchSize int) int {
	if batchSize <= 0 {
		batchSize = dm.GetBatchSize()
	}

	totalImages := len(dm.Images)
	if totalImages == 0 {
		return 0
	}

	totalBatches := totalImages / batchSize
	if totalImages%batchSize != 0 {
		totalBatches++ // 向上取整
	}

	return totalBatches
}

// GetImageBatch 获取指定批次的图像数据
func (dm *DataManager) GetImageBatch(batchID int, batchSize int) [][]float64 {
	if batchSize <= 0 {
		batchSize = dm.GetBatchSize()
	}

	totalImages := len(dm.Images)
	if totalImages == 0 {
		return [][]float64{}
	}

	// 计算批次范围
	startIdx := batchID * batchSize
	endIdx := startIdx + batchSize

	// 边界检查
	if startIdx >= totalImages {
		return [][]float64{} // 超出范围，返回空批次
	}

	if endIdx > totalImages {
		endIdx = totalImages // 调整结束索引
	}

	// 提取批次数据
	batchImages := dm.Images[startIdx:endIdx]

	// 如果批次不足，用零向量填充
	if len(batchImages) < batchSize {
		padding := make([][]float64, batchSize-len(batchImages))
		for i := range padding {
			padding[i] = make([]float64, len(dm.Images[0])) // 使用第一张图像的特征数
		}
		batchImages = append(batchImages, padding...)
	}

	return batchImages
}

// GetLabelBatch 获取指定批次的标签数据
func (dm *DataManager) GetLabelBatch(batchID int, batchSize int) []int {
	if batchSize <= 0 {
		batchSize = dm.GetBatchSize()
	}

	totalLabels := len(dm.Labels)
	if totalLabels == 0 {
		return []int{}
	}

	// 计算批次范围
	startIdx := batchID * batchSize
	endIdx := startIdx + batchSize

	// 边界检查
	if startIdx >= totalLabels {
		return []int{} // 超出范围，返回空批次
	}

	if endIdx > totalLabels {
		endIdx = totalLabels // 调整结束索引
	}

	// 提取批次数据
	batchLabels := dm.Labels[startIdx:endIdx]

	// 如果批次不足，用-1填充（表示无效标签）
	if len(batchLabels) < batchSize {
		padding := make([]int, batchSize-len(batchLabels))
		for i := range padding {
			padding[i] = -1 // 使用-1表示无效标签
		}
		batchLabels = append(batchLabels, padding...)
	}

	return batchLabels
}
