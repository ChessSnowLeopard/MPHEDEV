package hidden_layer

import (
	"MPHEDev/pkg/core/participant/types"
	"MPHEDev/pkg/core/participant/utils"
	"fmt"
	"sync"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// ReceiveService 隐藏层接收服务
type ReceiveService struct {
	participantID int
	receivedData  map[int]map[int]*types.LayerCommunicationData // [SourceID][BatchID]Data
	receiveStatus map[int]ReceiveStatus                         // [BatchID]Status
	statusMu      sync.RWMutex
}

// NewReceiveService 创建新的接收服务
func NewReceiveService(participantID int) *ReceiveService {
	return &ReceiveService{
		participantID: participantID,
		receivedData:  make(map[int]map[int]*types.LayerCommunicationData),
		receiveStatus: make(map[int]ReceiveStatus),
	}
}

// ReceiveFeaturesFromInputLayer 接收输入层特征数据
func (rs *ReceiveService) ReceiveFeaturesFromInputLayer(commData *types.LayerCommunicationData) error {
	// 1. 更新接收状态为receiving
	rs.updateReceiveStatus(commData.BatchID, ReceiveStatusReceiving, commData.SourceID, nil)

	// 2. 验证数据完整性
	if err := commData.ValidateData(); err != nil {
		rs.updateReceiveStatus(commData.BatchID, ReceiveStatusFailed, commData.SourceID, err)
		return fmt.Errorf("数据验证失败: %v", err)
	}

	// 3. 验证数据来源和目标
	if commData.TargetID != rs.participantID {
		rs.updateReceiveStatus(commData.BatchID, ReceiveStatusFailed, commData.SourceID,
			fmt.Errorf("目标参与方ID不匹配，期望: %d，实际: %d", rs.participantID, commData.TargetID))
		return fmt.Errorf("目标参与方ID不匹配")
	}

	if commData.Metadata.LayerType != types.LayerTypeInput {
		rs.updateReceiveStatus(commData.BatchID, ReceiveStatusFailed, commData.SourceID,
			fmt.Errorf("源层类型错误，期望: %s，实际: %s", types.LayerTypeInput, commData.Metadata.LayerType))
		return fmt.Errorf("源层类型错误")
	}

	if commData.Metadata.DataType != types.DataTypeFeatures {
		rs.updateReceiveStatus(commData.BatchID, ReceiveStatusFailed, commData.SourceID,
			fmt.Errorf("数据类型错误，期望: %s，实际: %s", types.DataTypeFeatures, commData.Metadata.DataType))
		return fmt.Errorf("数据类型错误")
	}

	// 4. 存储接收到的数据
	rs.statusMu.Lock()
	if rs.receivedData[commData.SourceID] == nil {
		rs.receivedData[commData.SourceID] = make(map[int]*types.LayerCommunicationData)
	}
	rs.receivedData[commData.SourceID][commData.BatchID] = commData
	rs.statusMu.Unlock()

	// 5. 更新接收状态为received
	rs.updateReceiveStatus(commData.BatchID, ReceiveStatusReceived, commData.SourceID, nil)

	fmt.Printf("✓ 隐藏层收到来自参与方 %d 批次 %d 的特征数据，共 %d 个密文向量\n",
		commData.SourceID, commData.BatchID, len(commData.Ciphertexts))

	return nil
}

// GetReceiveStatus 获取接收状态
func (rs *ReceiveService) GetReceiveStatus(batchID int) ReceiveStatus {
	rs.statusMu.RLock()
	defer rs.statusMu.RUnlock()

	if status, exists := rs.receiveStatus[batchID]; exists {
		return status
	}

	return ReceiveStatus{
		BatchID:   batchID,
		Status:    ReceiveStatusReceiving,
		Timestamp: time.Now(),
	}
}

// ValidateReceivedData 验证接收数据完整性
func (rs *ReceiveService) ValidateReceivedData(batchID int) bool {
	rs.statusMu.RLock()
	defer rs.statusMu.RUnlock()

	// 检查是否有接收状态
	status, exists := rs.receiveStatus[batchID]
	if !exists {
		return false
	}

	// 检查状态是否为received或validated
	if status.Status != ReceiveStatusReceived && status.Status != ReceiveStatusValidated {
		return false
	}

	// 检查是否有对应的数据
	for _, sourceData := range rs.receivedData {
		if data, exists := sourceData[batchID]; exists {
			// 验证数据完整性
			if err := data.ValidateData(); err != nil {
				return false
			}
			return true
		}
	}

	return false
}

// ProcessReceivedFeatures 处理接收到的特征数据
func (rs *ReceiveService) ProcessReceivedFeatures(batchID int) error {
	// 1. 验证数据完整性
	if !rs.ValidateReceivedData(batchID) {
		return fmt.Errorf("批次 %d 的数据验证失败", batchID)
	}

	// 2. 获取接收到的数据
	rs.statusMu.RLock()
	var commData *types.LayerCommunicationData
	for _, sourceData := range rs.receivedData {
		if data, exists := sourceData[batchID]; exists {
			commData = data
			break
		}
	}
	rs.statusMu.RUnlock()

	if commData == nil {
		return fmt.Errorf("批次 %d 的数据不存在", batchID)
	}

	// 3. 反序列化密文数据
	ciphertexts := make([]*rlwe.Ciphertext, len(commData.Ciphertexts))
	for i, cv := range commData.Ciphertexts {
		ct, err := rs.deserializeCiphertext(cv.Ciphertext, nil) // 暂时传入nil，实际应该传入参数
		if err != nil {
			rs.updateReceiveStatus(batchID, ReceiveStatusFailed, commData.SourceID, err)
			return fmt.Errorf("反序列化密文 %d 失败: %v", i, err)
		}
		ciphertexts[i] = ct
	}

	// 4. 更新状态为validated
	rs.updateReceiveStatus(batchID, ReceiveStatusValidated, commData.SourceID, nil)

	fmt.Printf("✓ 隐藏层处理批次 %d 特征数据完成，共处理 %d 个密文\n", batchID, len(ciphertexts))

	// 5. 这里可以添加隐藏层特定的处理逻辑
	// 例如：权重计算、激活函数应用等

	return nil
}

// ConvertToComputationFormat 转换为计算引擎格式
func (rs *ReceiveService) ConvertToComputationFormat(batchID int, params interface{}) (map[uint64]*rlwe.Ciphertext, error) {
	// 1. 获取已处理的数据（遍历所有数据源）
	rs.statusMu.RLock()
	var commData *types.LayerCommunicationData
	for sourceID, sourceData := range rs.receivedData {
		if data, exists := sourceData[batchID]; exists {
			commData = data
			fmt.Printf("找到批次 %d 的数据，来源参与方: %d\n", batchID, sourceID)
			break
		}
	}
	rs.statusMu.RUnlock()

	if commData == nil {
		return nil, fmt.Errorf("批次 %d 的数据不存在", batchID)
	}

	// 2. 反序列化密文
	ciphertexts := make([]*rlwe.Ciphertext, len(commData.Ciphertexts))
	for i, cv := range commData.Ciphertexts {
		// 使用传入的参数进行反序列化
		ct, err := rs.deserializeCiphertext(cv.Ciphertext, params)
		if err != nil {
			return nil, fmt.Errorf("反序列化密文 %d 失败: %v", i, err)
		}
		ciphertexts[i] = ct
	}

	// 3. 转换为计算引擎格式
	encryptedVectors := make(map[uint64]*rlwe.Ciphertext)
	for i, ct := range ciphertexts {
		encryptedVectors[uint64(i)] = ct
	}

	fmt.Printf("✓ 数据格式转换完成，共 %d 个密文向量\n", len(encryptedVectors))
	return encryptedVectors, nil
}

// deserializeCiphertext 反序列化单个密文
func (rs *ReceiveService) deserializeCiphertext(ciphertextStr string, params interface{}) (*rlwe.Ciphertext, error) {
	// 使用与message_processor.go相同的方法：先解码base64，再反序列化
	ctBytes, err := utils.DecodeFromBase64(ciphertextStr)
	if err != nil {
		return nil, fmt.Errorf("base64解码失败: %v", err)
	}

	var ct rlwe.Ciphertext
	if err := utils.DecodeShare(ctBytes, &ct); err != nil {
		return nil, fmt.Errorf("密文反序列化失败: %v", err)
	}

	return &ct, nil
}

// GetReceivedData 获取接收到的数据
func (rs *ReceiveService) GetReceivedData(sourceID, batchID int) (*types.LayerCommunicationData, bool) {
	rs.statusMu.RLock()
	defer rs.statusMu.RUnlock()

	if sourceData, exists := rs.receivedData[sourceID]; exists {
		if data, exists := sourceData[batchID]; exists {
			return data, true
		}
	}
	return nil, false
}

// GetAllReceivedData 获取所有接收到的数据
func (rs *ReceiveService) GetAllReceivedData() map[int]map[int]*types.LayerCommunicationData {
	rs.statusMu.RLock()
	defer rs.statusMu.RUnlock()

	result := make(map[int]map[int]*types.LayerCommunicationData)
	for sourceID, sourceData := range rs.receivedData {
		result[sourceID] = make(map[int]*types.LayerCommunicationData)
		for batchID, data := range sourceData {
			result[sourceID][batchID] = data
		}
	}
	return result
}

// updateReceiveStatus 更新接收状态
func (rs *ReceiveService) updateReceiveStatus(batchID int, status string, sourceID int, err error) {
	rs.statusMu.Lock()
	defer rs.statusMu.Unlock()

	rs.receiveStatus[batchID] = ReceiveStatus{
		BatchID:   batchID,
		Status:    status,
		Timestamp: time.Now(),
		Error:     err,
		SourceID:  sourceID,
		DataCount: 0, // 可以从实际数据中获取
		MessageID: fmt.Sprintf("recv_%d_%d", batchID, time.Now().UnixNano()),
	}
}
