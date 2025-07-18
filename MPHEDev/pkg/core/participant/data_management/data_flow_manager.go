package data_management

import (
	"MPHEDev/pkg/core/participant/types"
	"fmt"
	"log"
	"sync"
	"time"
)

// DataFlowManager 数据流管理器
type DataFlowManager struct {
	// 系统配置
	SystemConfig types.SystemConfig

	// 数据流控制
	FlowControl *types.DataFlowControl

	// 消息队列
	MessageQueue map[string][]*types.LayerMessage

	// 状态管理
	mu sync.RWMutex

	// 回调函数
	OnMessageReceived func(*types.LayerMessage) error
	OnBatchCompleted  func(int) error
	OnError           func(*types.ErrorInfo) error
}

// NewDataFlowManager 创建数据流管理器
func NewDataFlowManager(config types.SystemConfig) *DataFlowManager {
	return &DataFlowManager{
		SystemConfig: config,
		FlowControl: &types.DataFlowControl{
			CurrentBatch: 0,
			TotalBatches: 0,
			LayerStatus:  make(map[string]string),
			BatchStatus:  make(map[int]types.BatchStatus),
		},
		MessageQueue: make(map[string][]*types.LayerMessage),
	}
}

// ==================== 数据流控制方法 ====================

// StartBatch 开始处理批次
func (dfm *DataFlowManager) StartBatch(batchIndex int) error {
	dfm.mu.Lock()
	defer dfm.mu.Unlock()

	// 创建批次状态
	batchStatus := types.BatchStatus{
		BatchIndex:      batchIndex,
		InputLayerDone:  false,
		HiddenLayerDone: false,
		OutputLayerDone: false,
		LossComputed:    false,
		StartTime:       time.Now().UnixNano(),
	}

	dfm.FlowControl.BatchStatus[batchIndex] = batchStatus
	dfm.FlowControl.CurrentBatch = batchIndex

	log.Printf("开始处理批次 %d", batchIndex)
	return nil
}

// CompleteLayer 完成层处理
func (dfm *DataFlowManager) CompleteLayer(batchIndex int, layerType string) error {
	dfm.mu.Lock()
	defer dfm.mu.Unlock()

	batchStatus, exists := dfm.FlowControl.BatchStatus[batchIndex]
	if !exists {
		return fmt.Errorf("批次 %d 不存在", batchIndex)
	}

	// 更新层状态
	switch layerType {
	case types.LayerTypeInput:
		batchStatus.InputLayerDone = true
	case types.LayerTypeHidden:
		batchStatus.HiddenLayerDone = true
	case types.LayerTypeOutput:
		batchStatus.OutputLayerDone = true
	}

	dfm.FlowControl.BatchStatus[batchIndex] = batchStatus
	dfm.FlowControl.LayerStatus[layerType] = types.StatusCompleted

	log.Printf("批次 %d 的 %s 层处理完成", batchIndex, layerType)

	// 检查批次是否完成
	if dfm.isBatchCompleted(batchIndex) {
		return dfm.completeBatch(batchIndex)
	}

	return nil
}

// CompleteBatch 完成批次处理
func (dfm *DataFlowManager) CompleteBatch(batchIndex int) error {
	dfm.mu.Lock()
	defer dfm.mu.Unlock()

	return dfm.completeBatch(batchIndex)
}

// completeBatch 完成批次处理（内部方法）
func (dfm *DataFlowManager) completeBatch(batchIndex int) error {
	batchStatus := dfm.FlowControl.BatchStatus[batchIndex]
	batchStatus.LossComputed = true
	batchStatus.EndTime = time.Now().UnixNano()
	batchStatus.Duration = batchStatus.EndTime - batchStatus.StartTime

	dfm.FlowControl.BatchStatus[batchIndex] = batchStatus

	log.Printf("批次 %d 处理完成，用时: %d ms", batchIndex, batchStatus.Duration/int64(time.Millisecond))

	// 调用回调函数
	if dfm.OnBatchCompleted != nil {
		return dfm.OnBatchCompleted(batchIndex)
	}

	return nil
}

// isBatchCompleted 检查批次是否完成
func (dfm *DataFlowManager) isBatchCompleted(batchIndex int) bool {
	batchStatus := dfm.FlowControl.BatchStatus[batchIndex]
	return batchStatus.InputLayerDone && batchStatus.HiddenLayerDone && batchStatus.OutputLayerDone
}

// ==================== 消息处理方法 ====================

// SendMessage 发送层间消息
func (dfm *DataFlowManager) SendMessage(message *types.LayerMessage) error {
	dfm.mu.Lock()
	defer dfm.mu.Unlock()

	// 添加到消息队列
	queueKey := fmt.Sprintf("%s_%s", message.SourceLayer, message.TargetLayer)
	dfm.MessageQueue[queueKey] = append(dfm.MessageQueue[queueKey], message)

	log.Printf("发送消息: %s -> %s, 批次: %d", message.SourceLayer, message.TargetLayer, message.BatchIndex)

	// 调用回调函数
	if dfm.OnMessageReceived != nil {
		return dfm.OnMessageReceived(message)
	}

	return nil
}

// ReceiveMessage 接收层间消息
func (dfm *DataFlowManager) ReceiveMessage(sourceLayer, targetLayer string) (*types.LayerMessage, error) {
	dfm.mu.Lock()
	defer dfm.mu.Unlock()

	queueKey := fmt.Sprintf("%s_%s", sourceLayer, targetLayer)
	messages, exists := dfm.MessageQueue[queueKey]
	if !exists || len(messages) == 0 {
		return nil, fmt.Errorf("没有来自 %s 到 %s 的消息", sourceLayer, targetLayer)
	}

	// 取出第一个消息
	message := messages[0]
	dfm.MessageQueue[queueKey] = messages[1:]

	// 更新消息状态
	message.Metadata.Status = types.StatusProcessing

	log.Printf("接收消息: %s -> %s, 批次: %d", sourceLayer, targetLayer, message.BatchIndex)

	return message, nil
}

// GetMessageCount 获取消息数量
func (dfm *DataFlowManager) GetMessageCount(sourceLayer, targetLayer string) int {
	dfm.mu.RLock()
	defer dfm.mu.RUnlock()

	queueKey := fmt.Sprintf("%s_%s", sourceLayer, targetLayer)
	messages, exists := dfm.MessageQueue[queueKey]
	if !exists {
		return 0
	}

	return len(messages)
}

// ==================== 状态查询方法 ====================

// GetBatchStatus 获取批次状态
func (dfm *DataFlowManager) GetBatchStatus(batchIndex int) (types.BatchStatus, bool) {
	dfm.mu.RLock()
	defer dfm.mu.RUnlock()

	status, exists := dfm.FlowControl.BatchStatus[batchIndex]
	return status, exists
}

// GetLayerStatus 获取层状态
func (dfm *DataFlowManager) GetLayerStatus(layerType string) string {
	dfm.mu.RLock()
	defer dfm.mu.RUnlock()

	status, exists := dfm.FlowControl.LayerStatus[layerType]
	if !exists {
		return types.StatusPending
	}

	return status
}

// GetFlowControl 获取数据流控制信息
func (dfm *DataFlowManager) GetFlowControl() *types.DataFlowControl {
	dfm.mu.RLock()
	defer dfm.mu.RUnlock()

	return dfm.FlowControl
}

// ==================== 错误处理方法 ====================

// HandleError 处理错误
func (dfm *DataFlowManager) HandleError(errorInfo *types.ErrorInfo) error {
	dfm.mu.Lock()
	defer dfm.mu.Unlock()

	dfm.FlowControl.ErrorInfo = errorInfo
	dfm.FlowControl.LayerStatus[errorInfo.Layer] = types.StatusError

	log.Printf("处理错误: %s - %s", errorInfo.ErrorCode, errorInfo.ErrorMessage)

	// 调用错误回调函数
	if dfm.OnError != nil {
		return dfm.OnError(errorInfo)
	}

	return nil
}

// ==================== 数据流协调方法 ====================

// CoordinateInputToHidden 协调输入层到隐藏层的数据流
func (dfm *DataFlowManager) CoordinateInputToHidden(inputData *types.InputLayerData) error {
	// 创建消息
	message := types.NewLayerMessage(
		types.MessageTypeInputToHidden,
		types.LayerTypeInput,
		types.LayerTypeHidden,
		inputData.BatchIndex,
		inputData,
	)

	// 发送消息
	if err := dfm.SendMessage(message); err != nil {
		return fmt.Errorf("发送输入层到隐藏层消息失败: %v", err)
	}

	// 更新状态
	return dfm.CompleteLayer(inputData.BatchIndex, types.LayerTypeInput)
}

// CoordinateHiddenToOutput 协调隐藏层到输出层的数据流
func (dfm *DataFlowManager) CoordinateHiddenToOutput(hiddenData *types.HiddenLayerData) error {
	// 创建消息
	message := types.NewLayerMessage(
		types.MessageTypeHiddenToOutput,
		types.LayerTypeHidden,
		types.LayerTypeOutput,
		0, // 批次索引从hiddenData中获取
		hiddenData,
	)

	// 发送消息
	if err := dfm.SendMessage(message); err != nil {
		return fmt.Errorf("发送隐藏层到输出层消息失败: %v", err)
	}

	// 更新状态
	return dfm.CompleteLayer(0, types.LayerTypeHidden) // 这里需要从hiddenData获取批次索引
}

// CoordinateOutputToLoss 协调输出层到损失计算的数据流
func (dfm *DataFlowManager) CoordinateOutputToLoss(outputData *types.OutputLayerData) error {
	// 创建消息
	message := types.NewLayerMessage(
		types.MessageTypeOutputToLoss,
		types.LayerTypeOutput,
		"loss_computation",
		0, // 批次索引从outputData中获取
		outputData,
	)

	// 发送消息
	if err := dfm.SendMessage(message); err != nil {
		return fmt.Errorf("发送输出层到损失计算消息失败: %v", err)
	}

	// 更新状态
	return dfm.CompleteLayer(0, types.LayerTypeOutput) // 这里需要从outputData获取批次索引
}

// ==================== 工具方法 ====================

// PrintStatus 打印状态信息
func (dfm *DataFlowManager) PrintStatus() {
	dfm.mu.RLock()
	defer dfm.mu.RUnlock()

	fmt.Println("\n=== 数据流状态 ===")
	fmt.Printf("当前批次: %d\n", dfm.FlowControl.CurrentBatch)
	fmt.Printf("总批次数: %d\n", dfm.FlowControl.TotalBatches)

	fmt.Println("\n层状态:")
	for layer, status := range dfm.FlowControl.LayerStatus {
		fmt.Printf("• %s: %s\n", layer, status)
	}

	fmt.Println("\n批次状态:")
	for batchIndex, status := range dfm.FlowControl.BatchStatus {
		fmt.Printf("• 批次 %d: 输入层=%v, 隐藏层=%v, 输出层=%v, 损失=%v\n",
			batchIndex, status.InputLayerDone, status.HiddenLayerDone, status.OutputLayerDone, status.LossComputed)
	}

	fmt.Println("\n消息队列:")
	for queueKey, messages := range dfm.MessageQueue {
		fmt.Printf("• %s: %d 条消息\n", queueKey, len(messages))
	}

	if dfm.FlowControl.ErrorInfo != nil {
		fmt.Printf("\n错误信息: %s - %s\n", dfm.FlowControl.ErrorInfo.ErrorCode, dfm.FlowControl.ErrorInfo.ErrorMessage)
	}

	fmt.Println("==================")
}
