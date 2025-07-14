package input_layer

import (
	"MPHEDev/pkg/core/participant/types"
	"MPHEDev/pkg/core/participant/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// TransferService 输入层传输服务
type TransferService struct {
	participantID int
	peerManager   interface{} // 使用interface{}避免循环依赖
	client        *http.Client

	// 发送状态管理
	sendStatus map[int]SendStatus // [BatchID]SendStatus
	statusMu   sync.RWMutex
}

// NewTransferService 创建新的传输服务
func NewTransferService(participantID int, peerManager interface{}, client *http.Client) *TransferService {
	return &TransferService{
		participantID: participantID,
		peerManager:   peerManager,
		client:        client,
		sendStatus:    make(map[int]SendStatus),
	}
}

// SendFeaturesToHiddenLayer 发送特征数据给隐藏层
func (ts *TransferService) SendFeaturesToHiddenLayer(batchID int, targetID int, ciphertexts []*rlwe.Ciphertext) error {
	// 1. 更新发送状态为pending
	ts.updateSendStatus(batchID, SendStatusPending, targetID, nil)

	// 2. 序列化数据
	commData, err := ts.serializeFeaturesForHiddenLayer(batchID, targetID, ciphertexts)
	if err != nil {
		ts.updateSendStatus(batchID, SendStatusFailed, targetID, err)
		return fmt.Errorf("序列化特征数据失败: %v", err)
	}

	// 3. 验证数据完整性
	if err := commData.ValidateData(); err != nil {
		ts.updateSendStatus(batchID, SendStatusFailed, targetID, err)
		return fmt.Errorf("数据验证失败: %v", err)
	}

	// 4. HTTP发送
	if err := ts.sendHTTPRequest(targetID, commData); err != nil {
		ts.updateSendStatus(batchID, SendStatusFailed, targetID, err)
		return fmt.Errorf("HTTP发送失败: %v", err)
	}

	// 5. 更新发送状态为sent
	ts.updateSendStatus(batchID, SendStatusSent, targetID, nil)

	return nil
}

// serializeFeaturesForHiddenLayer 序列化特征数据用于隐藏层传输
func (ts *TransferService) serializeFeaturesForHiddenLayer(
	batchID int,
	targetID int,
	ciphertexts []*rlwe.Ciphertext,
) (*types.LayerCommunicationData, error) {
	// 使用通用数据结构
	commData := types.NewLayerCommunicationData(batchID, ts.participantID, targetID, types.LayerTypeInput, types.DataTypeFeatures)

	// 设置元数据信息
	commData.SetMetadataInfo(
		128,                         // batchSize - 从配置获取
		784,                         // featureCount - 从配置获取
		len(ciphertexts),            // vectorCount
		types.DataFormatInterleaved, // dataFormat
	)

	// 序列化每个密文向量
	for i, ct := range ciphertexts {
		// 使用现有的序列化工具
		serialized, err := utils.SerializeCiphertext(ct)
		if err != nil {
			return nil, fmt.Errorf("序列化密文 %d 失败: %v", i, err)
		}

		// 添加密文向量到通信数据
		commData.AddCiphertextVector(i, serialized, 64, 8192) // featureCount和slotCount从配置获取
	}

	// 添加额外信息
	commData.AddExtraInfo("source_layer", "input_layer")
	commData.AddExtraInfo("target_layer", "hidden_layer")
	commData.AddExtraInfo("encryption_method", "CKKS")

	return commData, nil
}

// sendHTTPRequest 发送HTTP请求
func (ts *TransferService) sendHTTPRequest(targetID int, commData *types.LayerCommunicationData) error {
	// 获取目标参与方的URL
	peerManager, ok := ts.peerManager.(interface{ GetPeers() map[int]string })
	if !ok {
		return fmt.Errorf("peerManager不支持GetPeers方法")
	}

	targetURL, exists := peerManager.GetPeers()[targetID]
	if !exists {
		return fmt.Errorf("参与方 %d 不在线或未找到", targetID)
	}

	// 序列化通信数据
	jsonData, err := json.Marshal(commData)
	if err != nil {
		return fmt.Errorf("序列化通信数据失败: %v", err)
	}

	// 构造请求体
	reqBody := map[string]interface{}{
		"from":    ts.participantID,
		"message": string(jsonData),
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %v", err)
	}

	// 发送HTTP POST请求
	resp, err := ts.client.Post(targetURL+"/message", "application/json", bytes.NewReader(reqJSON))
	if err != nil {
		return fmt.Errorf("发送特征数据到参与方 %d 失败: %v", targetID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("参与方 %d 返回错误状态码: %d", targetID, resp.StatusCode)
	}

	return nil
}

// GetSendStatus 获取发送状态
func (ts *TransferService) GetSendStatus(batchID int) SendStatus {
	ts.statusMu.RLock()
	defer ts.statusMu.RUnlock()

	if status, exists := ts.sendStatus[batchID]; exists {
		return status
	}

	return SendStatus{
		BatchID:   batchID,
		Status:    SendStatusPending,
		Timestamp: time.Now(),
	}
}

// RetrySend 重试发送
func (ts *TransferService) RetrySend(batchID int, targetID int, ciphertexts []*rlwe.Ciphertext) error {
	// 检查当前状态
	status := ts.GetSendStatus(batchID)
	if status.Status != SendStatusFailed {
		return fmt.Errorf("批次 %d 状态为 %s，无法重试", batchID, status.Status)
	}

	// 重新发送
	return ts.SendFeaturesToHiddenLayer(batchID, targetID, ciphertexts)
}

// updateSendStatus 更新发送状态
func (ts *TransferService) updateSendStatus(batchID int, status string, targetID int, err error) {
	ts.statusMu.Lock()
	defer ts.statusMu.Unlock()

	ts.sendStatus[batchID] = SendStatus{
		BatchID:   batchID,
		Status:    status,
		Timestamp: time.Now(),
		Error:     err,
		TargetID:  targetID,
		MessageID: fmt.Sprintf("msg_%d_%d", batchID, time.Now().UnixNano()),
	}
}
