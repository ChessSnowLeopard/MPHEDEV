package types

import (
	"fmt"
	"time"
)

// SyncPhase 同步阶段
type SyncPhase string

const (
	SyncPhaseDataCollection   SyncPhase = "data_collection"   // 数据收集阶段
	SyncPhaseFeatureTransfer  SyncPhase = "feature_transfer"  // 特征传输阶段
	SyncPhaseComputation      SyncPhase = "computation"       // 计算阶段
	SyncPhaseResultCollection SyncPhase = "result_collection" // 结果收集阶段
)

// SyncStatus 同步状态
type SyncStatus struct {
	Phase         SyncPhase         `json:"phase"`
	BatchID       int               `json:"batch_id"`
	ParticipantID int               `json:"participant_id"`
	Status        string            `json:"status"` // "waiting", "ready", "done", "failed"
	Timestamp     time.Time         `json:"timestamp"`
	Dependencies  map[int]bool      `json:"dependencies"`  // [ParticipantID]bool
	Confirmations map[int]time.Time `json:"confirmations"` // [ParticipantID]time.Time
	Timeout       time.Duration     `json:"timeout"`
	RetryCount    int               `json:"retry_count"`
	MaxRetries    int               `json:"max_retries"`
	Error         error             `json:"error,omitempty"`
}

// DoneMessage Done消息结构
type DoneMessage struct {
	Type         string    `json:"type"`         // "done", "ready", "confirm"
	Phase        SyncPhase `json:"phase"`        // 同步阶段
	BatchID      int       `json:"batch_id"`     // 批次ID
	SourceID     int       `json:"source_id"`    // 发送方ID
	TargetID     int       `json:"target_id"`    // 接收方ID
	MessageID    string    `json:"message_id"`   // 消息唯一标识
	Timestamp    time.Time `json:"timestamp"`    // 时间戳
	DataHash     string    `json:"data_hash"`    // 数据哈希（可选）
	Confirmation bool      `json:"confirmation"` // 是否为确认消息
}

// 同步状态常量
const (
	SyncStatusWaiting = "waiting"
	SyncStatusReady   = "ready"
	SyncStatusDone    = "done"
	SyncStatusFailed  = "failed"
)

// Done消息类型常量
const (
	DoneMessageTypeDone    = "done"
	DoneMessageTypeReady   = "ready"
	DoneMessageTypeConfirm = "confirm"
)

// 默认配置常量
const (
	DefaultSyncTimeout = 30 * time.Second
	DefaultMaxRetries  = 3
)

// NewSyncStatus 创建新的同步状态
func NewSyncStatus(phase SyncPhase, batchID, participantID int) *SyncStatus {
	return &SyncStatus{
		Phase:         phase,
		BatchID:       batchID,
		ParticipantID: participantID,
		Status:        SyncStatusWaiting,
		Timestamp:     time.Now(),
		Dependencies:  make(map[int]bool),
		Confirmations: make(map[int]time.Time),
		Timeout:       DefaultSyncTimeout,
		RetryCount:    0,
		MaxRetries:    DefaultMaxRetries,
	}
}

// NewDoneMessage 创建新的Done消息
func NewDoneMessage(msgType string, phase SyncPhase, batchID, sourceID, targetID int) *DoneMessage {
	return &DoneMessage{
		Type:         msgType,
		Phase:        phase,
		BatchID:      batchID,
		SourceID:     sourceID,
		TargetID:     targetID,
		MessageID:    generateDoneMessageID(),
		Timestamp:    time.Now(),
		DataHash:     "",
		Confirmation: false,
	}
}

// generateDoneMessageID 生成Done消息ID
func generateDoneMessageID() string {
	return fmt.Sprintf("done_%d", time.Now().UnixNano())
}

// GetSyncKey 获取同步状态键
func GetSyncKey(phase SyncPhase, batchID int) string {
	return fmt.Sprintf("%s_%d", phase, batchID)
}

// IsPhaseValid 检查同步阶段是否有效
func IsPhaseValid(phase SyncPhase) bool {
	switch phase {
	case SyncPhaseDataCollection, SyncPhaseFeatureTransfer, SyncPhaseComputation, SyncPhaseResultCollection:
		return true
	default:
		return false
	}
}

// IsStatusValid 检查同步状态是否有效
func IsStatusValid(status string) bool {
	switch status {
	case SyncStatusWaiting, SyncStatusReady, SyncStatusDone, SyncStatusFailed:
		return true
	default:
		return false
	}
}
