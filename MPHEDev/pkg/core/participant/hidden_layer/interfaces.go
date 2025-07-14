package hidden_layer

import (
	"MPHEDev/pkg/core/participant/types"
	"time"
)

// HiddenLayerReceiver 隐藏层接收器接口
type HiddenLayerReceiver interface {
	// 接收输入层特征数据
	ReceiveFeaturesFromInputLayer(commData *types.LayerCommunicationData) error

	// 获取接收状态
	GetReceiveStatus(batchID int) ReceiveStatus

	// 验证接收数据完整性
	ValidateReceivedData(batchID int) bool

	// 处理接收到的特征数据
	ProcessReceivedFeatures(batchID int) error
}

// ReceiveStatus 接收状态
type ReceiveStatus struct {
	BatchID   int       `json:"batch_id"`
	Status    string    `json:"status"` // "receiving", "received", "validated", "failed"
	Timestamp time.Time `json:"timestamp"`
	Error     error     `json:"error,omitempty"`
	SourceID  int       `json:"source_id"`
	DataCount int       `json:"data_count"` // 接收到的数据量
	MessageID string    `json:"message_id"`
}

// 接收状态常量
const (
	ReceiveStatusReceiving = "receiving"
	ReceiveStatusReceived  = "received"
	ReceiveStatusValidated = "validated"
	ReceiveStatusFailed    = "failed"
)
