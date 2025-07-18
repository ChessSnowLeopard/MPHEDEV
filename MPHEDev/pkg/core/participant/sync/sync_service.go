package sync

import (
	"MPHEDev/pkg/core/participant/types"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// SyncServiceImpl 同步服务实现
type SyncServiceImpl struct {
	participantID int
	peerManager   interface{}
	client        *http.Client

	// 同步状态管理
	syncStatus map[string]*types.SyncStatus // [Phase_BatchID]*SyncStatus
	statusMu   sync.RWMutex

	// 确认超时管理
	confirmationTimeout time.Duration
	maxRetries          int

	// 消息去重
	processedMessages map[string]bool // [MessageID]bool
	messageMu         sync.RWMutex
}

// NewSyncService 创建新的同步服务
func NewSyncService(participantID int, peerManager interface{}, client *http.Client) *SyncServiceImpl {
	return &SyncServiceImpl{
		participantID:       participantID,
		peerManager:         peerManager,
		client:              client,
		syncStatus:          make(map[string]*types.SyncStatus),
		confirmationTimeout: types.DefaultSyncTimeout,
		maxRetries:          types.DefaultMaxRetries,
		processedMessages:   make(map[string]bool),
	}
}

// SendDoneMessage 发送Done消息
func (ss *SyncServiceImpl) SendDoneMessage(phase types.SyncPhase, batchID int, targetID int) error {
	// 1. 验证参数
	if !types.IsPhaseValid(phase) {
		return fmt.Errorf("无效的同步阶段: %s", phase)
	}

	// 2. 创建Done消息
	doneMsg := types.NewDoneMessage(types.DoneMessageTypeDone, phase, batchID, ss.participantID, targetID)

	// 3. 发送消息
	return ss.sendDoneMessage(doneMsg)
}

// WaitForDoneConfirmation 等待Done确认
func (ss *SyncServiceImpl) WaitForDoneConfirmation(phase types.SyncPhase, batchID int, sourceID int, timeout time.Duration) error {
	// 1. 验证参数
	if !types.IsPhaseValid(phase) {
		return fmt.Errorf("无效的同步阶段: %s", phase)
	}

	// 2. 获取同步状态
	syncKey := types.GetSyncKey(phase, batchID)
	ss.statusMu.Lock()
	if ss.syncStatus[syncKey] == nil {
		ss.syncStatus[syncKey] = types.NewSyncStatus(phase, batchID, ss.participantID)
	}
	ss.statusMu.Unlock()

	// 3. 等待确认
	startTime := time.Now()
	for {
		// 检查是否已收到确认
		if ss.hasConfirmation(syncKey, sourceID) {
			return nil
		}

		// 检查超时
		if time.Since(startTime) > timeout {
			return fmt.Errorf("等待Done确认超时，阶段: %s, 批次: %d, 来源: %d", phase, batchID, sourceID)
		}

		// 等待一段时间后重试
		time.Sleep(100 * time.Millisecond)
	}
}

// HandleDoneMessage 处理接收到的Done消息
func (ss *SyncServiceImpl) HandleDoneMessage(doneMsg *types.DoneMessage) error {
	// 1. 消息去重检查
	ss.messageMu.Lock()
	if ss.processedMessages[doneMsg.MessageID] {
		ss.messageMu.Unlock()
		return fmt.Errorf("消息已处理: %s", doneMsg.MessageID)
	}
	ss.processedMessages[doneMsg.MessageID] = true
	ss.messageMu.Unlock()

	// 2. 验证消息
	if err := ss.validateDoneMessage(doneMsg); err != nil {
		return fmt.Errorf("Done消息验证失败: %v", err)
	}

	// 3. 更新同步状态
	syncKey := types.GetSyncKey(doneMsg.Phase, doneMsg.BatchID)
	ss.statusMu.Lock()
	if ss.syncStatus[syncKey] == nil {
		ss.syncStatus[syncKey] = types.NewSyncStatus(doneMsg.Phase, doneMsg.BatchID, ss.participantID)
	}
	syncStatus := ss.syncStatus[syncKey]
	syncStatus.Confirmations[doneMsg.SourceID] = doneMsg.Timestamp
	ss.statusMu.Unlock()

	// 4. 发送确认回复
	if !doneMsg.Confirmation {
		confirmMsg := types.NewDoneMessage(types.DoneMessageTypeConfirm, doneMsg.Phase, doneMsg.BatchID, ss.participantID, doneMsg.SourceID)
		confirmMsg.Confirmation = true
		return ss.sendDoneMessage(confirmMsg)
	}

	fmt.Printf("✓ 处理Done消息: 阶段=%s, 批次=%d, 来源=%d\n", doneMsg.Phase, doneMsg.BatchID, doneMsg.SourceID)
	return nil
}

// CheckSyncStatus 检查同步状态
func (ss *SyncServiceImpl) CheckSyncStatus(phase types.SyncPhase, batchID int) types.SyncStatus {
	syncKey := types.GetSyncKey(phase, batchID)
	ss.statusMu.RLock()
	defer ss.statusMu.RUnlock()

	if syncStatus, exists := ss.syncStatus[syncKey]; exists {
		return *syncStatus
	}

	// 返回默认状态
	return *types.NewSyncStatus(phase, batchID, ss.participantID)
}

// ResetSyncStatus 重置同步状态
func (ss *SyncServiceImpl) ResetSyncStatus(phase types.SyncPhase, batchID int) error {
	syncKey := types.GetSyncKey(phase, batchID)
	ss.statusMu.Lock()
	defer ss.statusMu.Unlock()

	ss.syncStatus[syncKey] = types.NewSyncStatus(phase, batchID, ss.participantID)
	return nil
}

// SetDependencies 设置依赖关系
func (ss *SyncServiceImpl) SetDependencies(phase types.SyncPhase, batchID int, dependencies map[int]bool) error {
	syncKey := types.GetSyncKey(phase, batchID)
	ss.statusMu.Lock()
	defer ss.statusMu.Unlock()

	if ss.syncStatus[syncKey] == nil {
		ss.syncStatus[syncKey] = types.NewSyncStatus(phase, batchID, ss.participantID)
	}

	ss.syncStatus[syncKey].Dependencies = dependencies
	return nil
}

// CheckDependenciesComplete 检查是否所有依赖都已完成
func (ss *SyncServiceImpl) CheckDependenciesComplete(phase types.SyncPhase, batchID int) bool {
	syncKey := types.GetSyncKey(phase, batchID)
	ss.statusMu.RLock()
	defer ss.statusMu.RUnlock()

	syncStatus, exists := ss.syncStatus[syncKey]
	if !exists {
		return false
	}

	for participantID, required := range syncStatus.Dependencies {
		if required {
			if _, confirmed := syncStatus.Confirmations[participantID]; !confirmed {
				return false
			}
		}
	}

	return true
}

// GetConfirmations 获取确认状态
func (ss *SyncServiceImpl) GetConfirmations(phase types.SyncPhase, batchID int) map[int]time.Time {
	syncKey := types.GetSyncKey(phase, batchID)
	ss.statusMu.RLock()
	defer ss.statusMu.RUnlock()

	if syncStatus, exists := ss.syncStatus[syncKey]; exists {
		result := make(map[int]time.Time)
		for k, v := range syncStatus.Confirmations {
			result[k] = v
		}
		return result
	}

	return make(map[int]time.Time)
}

// SetTimeout 设置超时配置
func (ss *SyncServiceImpl) SetTimeout(timeout time.Duration) {
	ss.confirmationTimeout = timeout
}

// SetMaxRetries 设置最大重试次数
func (ss *SyncServiceImpl) SetMaxRetries(maxRetries int) {
	ss.maxRetries = maxRetries
}

// sendDoneMessage 发送Done消息的内部方法
func (ss *SyncServiceImpl) sendDoneMessage(doneMsg *types.DoneMessage) error {
	// 1. 获取目标参与方的URL
	targetURL, exists := ss.getPeerURL(doneMsg.TargetID)
	if !exists {
		return fmt.Errorf("参与方 %d 不在线或未找到", doneMsg.TargetID)
	}

	// 2. 序列化Done消息
	doneMsgJSON, err := json.Marshal(doneMsg)
	if err != nil {
		return fmt.Errorf("序列化Done消息失败: %v", err)
	}

	// 3. 构造DataMessage格式的消息
	dataMsg := map[string]interface{}{
		"type": "sync_done", // 使用sync_done类型
		"from": ss.participantID,
		"data": string(doneMsgJSON), // 将DoneMessage作为data字段
	}

	// 4. 序列化DataMessage
	jsonData, err := json.Marshal(dataMsg)
	if err != nil {
		return fmt.Errorf("序列化DataMessage失败: %v", err)
	}

	// 5. 构造请求
	reqBody := map[string]interface{}{
		"from":    ss.participantID,
		"message": string(jsonData),
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %v", err)
	}

	// 6. 发送HTTP POST请求
	resp, err := ss.client.Post(targetURL+"/message", "application/json", bytes.NewReader(reqJSON))
	if err != nil {
		return fmt.Errorf("发送Done消息到参与方 %d 失败: %v", doneMsg.TargetID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("参与方 %d 返回错误状态码: %d", doneMsg.TargetID, resp.StatusCode)
	}

	fmt.Printf("✓ 发送Done消息: 阶段=%s, 批次=%d, 目标=%d\n", doneMsg.Phase, doneMsg.BatchID, doneMsg.TargetID)
	return nil
}

// validateDoneMessage 验证Done消息
func (ss *SyncServiceImpl) validateDoneMessage(doneMsg *types.DoneMessage) error {
	// 1. 检查目标ID
	if doneMsg.TargetID != ss.participantID {
		return fmt.Errorf("目标参与方ID不匹配，期望: %d，实际: %d", ss.participantID, doneMsg.TargetID)
	}

	// 2. 检查同步阶段
	if !types.IsPhaseValid(doneMsg.Phase) {
		return fmt.Errorf("无效的同步阶段: %s", doneMsg.Phase)
	}

	// 3. 检查消息类型
	switch doneMsg.Type {
	case types.DoneMessageTypeDone, types.DoneMessageTypeReady, types.DoneMessageTypeConfirm:
		// 有效类型
	default:
		return fmt.Errorf("无效的消息类型: %s", doneMsg.Type)
	}

	// 4. 检查时间戳（可选：检查消息是否过期）
	if time.Since(doneMsg.Timestamp) > 5*time.Minute {
		return fmt.Errorf("消息已过期，时间戳: %v", doneMsg.Timestamp)
	}

	return nil
}

// hasConfirmation 检查是否有确认
func (ss *SyncServiceImpl) hasConfirmation(syncKey string, sourceID int) bool {
	ss.statusMu.RLock()
	defer ss.statusMu.RUnlock()

	if syncStatus, exists := ss.syncStatus[syncKey]; exists {
		_, confirmed := syncStatus.Confirmations[sourceID]
		return confirmed
	}

	return false
}

// getPeerURL 获取对等节点URL
func (ss *SyncServiceImpl) getPeerURL(peerID int) (string, bool) {
	peerManager, ok := ss.peerManager.(interface{ GetPeers() map[int]string })
	if !ok {
		return "", false
	}

	peers := peerManager.GetPeers()
	url, exists := peers[peerID]
	return url, exists
}
