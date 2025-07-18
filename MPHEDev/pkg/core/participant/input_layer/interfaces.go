package input_layer

import "time"

// InputLayerSender 输入层发送器接口
type InputLayerSender interface {
	// 发送特征数据给隐藏层
	SendFeaturesToHiddenLayer(batchID int, targetID int) error

	// 获取发送状态
	GetSendStatus(batchID int) SendStatus

	// 重试发送（如果失败）
	RetrySend(batchID int) error
}

// SendStatus 发送状态
type SendStatus struct {
	BatchID   int       `json:"batch_id"`
	Status    string    `json:"status"` // "pending", "sent", "confirmed", "failed"
	Timestamp time.Time `json:"timestamp"`
	Error     error     `json:"error,omitempty"`
	TargetID  int       `json:"target_id"`
	MessageID string    `json:"message_id"`
}

// 发送状态常量
const (
	SendStatusPending   = "pending"
	SendStatusSent      = "sent"
	SendStatusConfirmed = "confirmed"
	SendStatusFailed    = "failed"
)
