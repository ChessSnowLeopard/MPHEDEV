package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"MPHEDev/pkg/core/participant/hidden_layer"
	"MPHEDev/pkg/core/participant/sync"
	"MPHEDev/pkg/core/participant/types"
	"MPHEDev/pkg/core/participant/utils"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// MessageProcessor 消息处理器
type MessageProcessor struct {
	participant *Participant
}

// NewMessageProcessor 创建新的消息处理器
func NewMessageProcessor(participant *Participant) *MessageProcessor {
	return &MessageProcessor{
		participant: participant,
	}
}

// GetSyncService 获取同步服务
func (mp *MessageProcessor) GetSyncService() sync.SyncService {
	if mp.participant != nil {
		return mp.participant.GetSyncService()
	}
	return nil
}

// HandleMessage 处理接收到的消息
func (mp *MessageProcessor) HandleMessage(fromID int, message string) {
	mp.participant.LogInfo(fmt.Sprintf("参与方 %d 收到来自参与方 %d 的消息", mp.participant.ID, fromID))

	// 解析消息
	var dataMsg DataMessage
	if err := json.Unmarshal([]byte(message), &dataMsg); err != nil {
		mp.participant.LogError(fmt.Sprintf("解析消息失败: %v", err))
		return
	}

	switch dataMsg.Type {
	case "feature_batch":
		mp.handleFeatureBatch(fromID, dataMsg)
	case "label_batch":
		mp.handleLabelBatch(fromID, dataMsg)
	case "hidden_output_batch":
		mp.handleHiddenOutputBatch(fromID, dataMsg)
	case "done":
		mp.handleDoneMessage(fromID, dataMsg)
	case "sync_done":
		mp.handleSyncDoneMessage(fromID, dataMsg)
	case "input_done":
		mp.participant.DataManager.SetInputLayerDone(true)
		fmt.Printf("参与方 %d 收到输入层完成消息\n", mp.participant.ID)
	case "output_done":
		mp.participant.DataManager.SetOutputLayerDone(true)
		fmt.Printf("参与方 %d 收到输出层完成消息\n", mp.participant.ID)
	default:
		fmt.Printf("未知消息类型: %s\n", dataMsg.Type)
	}
}

// handleFeatureBatch 处理特征数据批次
func (mp *MessageProcessor) handleFeatureBatch(fromID int, dataMsg DataMessage) {
	fmt.Printf("参与方 %d 处理来自参与方 %d 的特征数据批次\n", mp.participant.ID, fromID)

	// 解析批次信息
	batchInfoBytes, err := utils.DecodeFromBase64(dataMsg.Data)
	if err != nil {
		fmt.Printf("解析批次信息失败: %v\n", err)
		return
	}
	batchInfo := string(batchInfoBytes)
	parts := strings.Split(batchInfo, ",")
	if len(parts) != 2 {
		fmt.Printf("批次信息格式错误: %s\n", batchInfo)
		return
	}

	batchIndex, err := strconv.Atoi(parts[0])
	if err != nil {
		fmt.Printf("解析批次索引失败: %v\n", err)
		return
	}
	totalBatches, err := strconv.Atoi(parts[1])
	if err != nil {
		fmt.Printf("解析总批次数失败: %v\n", err)
		return
	}

	// 初始化批次状态
	featureBatchStatus := mp.participant.DataManager.GetFeatureBatchStatus()
	if featureBatchStatus[fromID] == nil {
		featureBatchStatus[fromID] = &types.BatchStatus{
			BatchIndex:      batchIndex,
			InputLayerDone:  false,
			HiddenLayerDone: false,
			OutputLayerDone: false,
			LossComputed:    false,
			StartTime:       0,
		}
	}

	// 存储密文数据
	receivedFeatureCiphertexts := mp.participant.DataManager.GetReceivedFeatureCiphertexts()
	if receivedFeatureCiphertexts[fromID] == nil {
		receivedFeatureCiphertexts[fromID] = make([][]string, totalBatches)
	}

	// 边界检查
	if batchIndex < 0 || batchIndex >= totalBatches {
		fmt.Printf("错误：批次索引 %d 超出范围 [0, %d)\n", batchIndex, totalBatches)
		return
	}

	receivedFeatureCiphertexts[fromID][batchIndex] = dataMsg.BatchData

	// 反序列化密文数据并通知输入层处理器
	if mp.participant.DataManager.IsInputLayer() {
		// 将base64字符串反序列化为密文对象
		deserializedCiphertexts := make([]*rlwe.Ciphertext, len(dataMsg.BatchData))
		for i, serialized := range dataMsg.BatchData {
			// 使用与decryption_service相同的方法：先解码base64，再反序列化
			ctBytes, err := utils.DecodeFromBase64(serialized)
			if err != nil {
				fmt.Printf("反序列化密文 %d 失败: base64解码失败: %v\n", i, err)
				continue
			}
			var ct rlwe.Ciphertext
			if err := utils.DecodeShare(ctBytes, &ct); err != nil {
				fmt.Printf("反序列化密文 %d 失败: 密文反序列化失败: %v\n", i, err)
				continue
			}
			deserializedCiphertexts[i] = &ct
		}

		// 通知输入层处理器处理接收到的特征数据
		fmt.Printf("调试：尝试获取输入层处理器...\n")
		if inputProcessor, ok := mp.participant.GetInputLayerProcessor(); ok {
			fmt.Printf("调试：成功获取输入层处理器，类型: %T\n", inputProcessor)
			if err := inputProcessor.HandleFeatureData(fromID, batchIndex, deserializedCiphertexts); err != nil {
				fmt.Printf("输入层处理特征数据失败: %v\n", err)
			} else {
				fmt.Printf("调试：输入层处理特征数据成功\n")
			}
		} else {
			fmt.Printf("调试：获取输入层处理器失败\n")
		}
	} else if mp.participant.DataManager.IsHiddenLayer() {
		// 隐藏层处理接收到的特征数据
		fmt.Printf("隐藏层处理来自参与方 %d 的特征数据批次 %d\n", fromID, batchIndex)

		// 构造LayerCommunicationData
		commData := &types.LayerCommunicationData{
			BatchID:     batchIndex,
			SourceID:    fromID,
			TargetID:    mp.participant.ID,
			Timestamp:   time.Now(),
			Ciphertexts: make([]types.CiphertextVector, len(dataMsg.BatchData)),
			Metadata: types.LayerMetadata{
				LayerType:    "input_layer",
				DataType:     "features",
				BatchSize:    1,
				FeatureCount: len(dataMsg.BatchData),
				VectorCount:  len(dataMsg.BatchData),
				DataFormat:   "interleaved_features",
				ExtraInfo:    make(map[string]interface{}),
			},
		}

		// 填充密文向量数据
		for i, serialized := range dataMsg.BatchData {
			commData.Ciphertexts[i] = types.CiphertextVector{
				VectorID:     i,
				Ciphertext:   serialized,
				FeatureCount: 1,
				SlotCount:    8192, // 默认值，实际应该从配置获取
			}
		}

		// 通知隐藏层处理器处理接收到的特征数据
		if hiddenProcessor, ok := mp.participant.LayerProcessor.(*hidden_layer.HiddenLayerProcessor); ok {
			if err := hiddenProcessor.ReceiveFeaturesFromInputLayer(commData); err != nil {
				fmt.Printf("隐藏层处理特征数据失败: %v\n", err)
			}
		} else {
			// 尝试通过接口调用
			if layerProcessor, ok := mp.participant.LayerProcessor.(interface {
				ReceiveFeaturesFromInputLayer(*types.LayerCommunicationData) error
			}); ok {
				if err := layerProcessor.ReceiveFeaturesFromInputLayer(commData); err != nil {
					fmt.Printf("隐藏层处理特征数据失败: %v\n", err)
				}
			}
		}
	}

	// 标记批次已接收
	fmt.Printf("参与方 %d 已接收参与方 %d 的特征批次 %d/%d\n", mp.participant.ID, fromID, batchIndex, totalBatches)

	// 检查是否所有批次都已接收
	receivedFeatures := mp.participant.DataManager.GetReceivedFeatures()
	receivedFeatures[fromID] = true
	fmt.Printf("参与方 %d 已接收参与方 %d 的所有特征数据\n", mp.participant.ID, fromID)
}

// handleLabelBatch 处理标签数据批次
func (mp *MessageProcessor) handleLabelBatch(fromID int, dataMsg DataMessage) {
	fmt.Printf("参与方 %d 处理来自参与方 %d 的标签数据批次\n", mp.participant.ID, fromID)

	// 解析批次信息
	batchInfoBytes, err := utils.DecodeFromBase64(dataMsg.Data)
	if err != nil {
		fmt.Printf("解析批次信息失败: %v\n", err)
		return
	}
	batchInfo := string(batchInfoBytes)
	parts := strings.Split(batchInfo, ",")
	if len(parts) != 2 {
		fmt.Printf("批次信息格式错误: %s\n", batchInfo)
		return
	}

	batchIndex, err := strconv.Atoi(parts[0])
	if err != nil {
		fmt.Printf("解析批次索引失败: %v\n", err)
		return
	}
	totalBatches, err := strconv.Atoi(parts[1])
	if err != nil {
		fmt.Printf("解析总批次数失败: %v\n", err)
		return
	}

	// 初始化批次状态
	labelBatchStatus := mp.participant.DataManager.GetLabelBatchStatus()
	if labelBatchStatus[fromID] == nil {
		labelBatchStatus[fromID] = &types.BatchStatus{
			BatchIndex:      batchIndex,
			InputLayerDone:  false,
			HiddenLayerDone: false,
			OutputLayerDone: false,
			LossComputed:    false,
			StartTime:       0,
		}
	}

	// 存储密文数据
	receivedLabelCiphertexts := mp.participant.DataManager.GetReceivedLabelCiphertexts()
	if receivedLabelCiphertexts[fromID] == nil {
		receivedLabelCiphertexts[fromID] = make([][]string, totalBatches)
	}

	// 边界检查
	if batchIndex < 0 || batchIndex >= totalBatches {
		fmt.Printf("错误：批次索引 %d 超出范围 [0, %d)\n", batchIndex, totalBatches)
		return
	}

	receivedLabelCiphertexts[fromID][batchIndex] = dataMsg.BatchData

	// 反序列化密文数据并通知输出层处理器
	if mp.participant.DataManager.IsOutputLayer() {
		// 将base64字符串反序列化为密文对象
		deserializedCiphertexts := make([]*rlwe.Ciphertext, len(dataMsg.BatchData))
		for i, serialized := range dataMsg.BatchData {
			// 使用与decryption_service相同的方法：先解码base64，再反序列化
			ctBytes, err := utils.DecodeFromBase64(serialized)
			if err != nil {
				fmt.Printf("反序列化密文 %d 失败: base64解码失败: %v\n", i, err)
				continue
			}
			var ct rlwe.Ciphertext
			if err := utils.DecodeShare(ctBytes, &ct); err != nil {
				fmt.Printf("反序列化密文 %d 失败: 密文反序列化失败: %v\n", i, err)
				continue
			}
			deserializedCiphertexts[i] = &ct
		}

		// 通知输出层处理器处理接收到的标签数据
		if outputProcessor, ok := mp.participant.GetOutputLayerProcessor(); ok {
			if err := outputProcessor.HandleLabelData(fromID, batchIndex, deserializedCiphertexts); err != nil {
				fmt.Printf("输出层处理标签数据失败: %v\n", err)
			}
		}
	}

	// 标记批次已接收
	fmt.Printf("参与方 %d 已接收参与方 %d 的标签批次 %d/%d\n", mp.participant.ID, fromID, batchIndex, totalBatches)

	// 检查是否所有批次都已接收
	receivedLabels := mp.participant.DataManager.GetReceivedLabels()
	receivedLabels[fromID] = true
	// 重要：更新数据管理器中的接收状态
	mp.participant.DataManager.SetReceivedLabels(receivedLabels)
	fmt.Printf("参与方 %d 已接收参与方 %d 的所有标签数据\n", mp.participant.ID, fromID)
}

// handleHiddenOutputBatch 处理隐藏层输出数据批次
func (mp *MessageProcessor) handleHiddenOutputBatch(fromID int, dataMsg DataMessage) {
	fmt.Printf("参与方 %d 处理来自参与方 %d 的隐藏层输出数据批次\n", mp.participant.ID, fromID)
	fmt.Printf("消息详情:\n")
	fmt.Printf("  • 消息类型: %s\n", dataMsg.Type)
	fmt.Printf("  • 来源参与方: %d\n", fromID)
	fmt.Printf("  • 批次数据数量: %d\n", len(dataMsg.BatchData))

	// 解析批次信息
	batchInfoBytes, err := utils.DecodeFromBase64(dataMsg.Data)
	if err != nil {
		fmt.Printf("解析批次信息失败: %v\n", err)
		return
	}
	batchInfo := string(batchInfoBytes)
	parts := strings.Split(batchInfo, ",")
	if len(parts) != 2 {
		fmt.Printf("批次信息格式错误: %s\n", batchInfo)
		return
	}

	batchIndex, err := strconv.Atoi(parts[0])
	if err != nil {
		fmt.Printf("解析批次索引失败: %v\n", err)
		return
	}
	totalBatches, err := strconv.Atoi(parts[1])
	if err != nil {
		fmt.Printf("解析总批次数失败: %v\n", err)
		return
	}

	fmt.Printf("  • 批次索引: %d\n", batchIndex)
	fmt.Printf("  • 总批次数: %d\n", totalBatches)

	// 边界检查
	if batchIndex < 0 || batchIndex >= totalBatches {
		fmt.Printf("错误：批次索引 %d 超出范围 [0, %d)\n", batchIndex, totalBatches)
		return
	}

	// 反序列化密文数据并通知输出层处理器
	if mp.participant.DataManager.IsOutputLayer() {
		fmt.Printf("✓ 当前参与方是输出层，开始处理隐藏层输出数据...\n")

		// 将base64字符串反序列化为密文对象（仿照标签处理方式）
		deserializedCiphertexts := make([]*rlwe.Ciphertext, 0, len(dataMsg.BatchData))
		successCount := 0
		for i, serialized := range dataMsg.BatchData {
			// 使用与decryption_service相同的方法：先解码base64，再反序列化
			ctBytes, err := utils.DecodeFromBase64(serialized)
			if err != nil {
				fmt.Printf("反序列化密文 %d 失败: base64解码失败: %v\n", i, err)
				continue
			}
			var ct rlwe.Ciphertext
			if err := utils.DecodeShare(ctBytes, &ct); err != nil {
				fmt.Printf("反序列化密文 %d 失败: 密文反序列化失败: %v\n", i, err)
				continue
			}
			deserializedCiphertexts = append(deserializedCiphertexts, &ct)
			successCount++
		}

		fmt.Printf("  • 成功反序列化密文数量: %d/%d\n", successCount, len(dataMsg.BatchData))

		// 检查反序列化结果
		if len(deserializedCiphertexts) == 0 {
			fmt.Printf("警告：所有密文反序列化失败，跳过处理\n")
			return
		}

		// 通知输出层处理器处理接收到的隐藏层输出数据（仿照标签处理方式）
		fmt.Printf("尝试获取输出层处理器...\n")
		if outputProcessor, ok := mp.participant.LayerProcessor.(interface {
			HandleHiddenLayerOutput(int, int, []*rlwe.Ciphertext) error
		}); ok {
			fmt.Printf("✓ 成功获取输出层处理器，开始处理隐藏层输出数据...\n")
			if err := outputProcessor.HandleHiddenLayerOutput(fromID, batchIndex, deserializedCiphertexts); err != nil {
				fmt.Printf("❌ 输出层处理隐藏层输出数据失败: %v\n", err)
			} else {
				fmt.Printf("✓ 输出层成功接收并处理隐藏层输出数据，批次 %d\n", batchIndex)
			}
		} else {
			fmt.Printf("❌ 输出层处理器不支持接收隐藏层输出接口\n")
			fmt.Printf("当前处理器类型: %T\n", mp.participant.LayerProcessor)
		}
	} else {
		fmt.Printf("非输出层参与方收到隐藏层输出数据，忽略\n")
		fmt.Printf("当前参与方角色: %s\n", mp.participant.DataManager.GetDataSplit())
	}

	// 标记批次已接收
	fmt.Printf("参与方 %d 已接收参与方 %d 的隐藏层输出批次 %d/%d\n", mp.participant.ID, fromID, batchIndex, totalBatches)
}

// handleDoneMessage 处理完成消息
func (mp *MessageProcessor) handleDoneMessage(fromID int, dataMsg DataMessage) {
	fmt.Printf("参与方 %d 收到来自参与方 %d 的完成消息\n", mp.participant.ID, fromID)
	// 可以在这里添加完成后的处理逻辑
}

// handleSyncDoneMessage 处理同步完成消息
func (mp *MessageProcessor) handleSyncDoneMessage(fromID int, dataMsg DataMessage) {
	fmt.Printf("参与方 %d 收到来自参与方 %d 的同步完成消息\n", mp.participant.ID, fromID)

	// 检查同步服务是否已初始化
	if mp.participant.SyncService == nil {
		fmt.Printf("警告: 同步服务未初始化，无法处理同步消息\n")
		return
	}

	// 解析Done消息
	var doneMsg types.DoneMessage
	if err := json.Unmarshal([]byte(dataMsg.Data), &doneMsg); err != nil {
		fmt.Printf("解析Done消息失败: %v\n", err)
		return
	}

	// 使用同步服务处理Done消息
	if err := mp.participant.SyncService.HandleDoneMessage(&doneMsg); err != nil {
		fmt.Printf("处理Done消息失败: %v\n", err)
		return
	}

	fmt.Printf("✓ 成功处理来自参与方 %d 的同步消息: 阶段=%s, 批次=%d\n",
		fromID, doneMsg.Phase, doneMsg.BatchID)
}

// SendMessageToParticipant 向指定参与方发送消息
func (mp *MessageProcessor) SendMessageToParticipant(targetID int, message string) error {
	// 获取目标参与方的URL
	targetURL, exists := mp.participant.PeerManager.GetPeers()[targetID]
	if !exists {
		return fmt.Errorf("参与方 %d 不在线或未找到", targetID)
	}

	// 构造请求
	reqBody := map[string]interface{}{
		"from":    mp.participant.ID,
		"message": message,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %v", err)
	}

	// 发送HTTP POST请求
	resp, err := mp.participant.Client.Client.Post(targetURL+"/message", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("发送消息到参与方 %d 失败: %v", targetID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("参与方 %d 返回错误状态码: %d", targetID, resp.StatusCode)
	}

	return nil
}

// SendCiphertextData 向指定参与方发送密文数据
func (mp *MessageProcessor) SendCiphertextData(targetID int, dataType string, batchID int, ciphertexts []*rlwe.Ciphertext) error {
	// 获取目标参与方的URL
	targetURL, exists := mp.participant.PeerManager.GetPeers()[targetID]
	if !exists {
		return fmt.Errorf("参与方 %d 不在线或未找到", targetID)
	}

	// 序列化密文数据（使用与decryption_service相同的方法）
	serializedCiphertexts := make([]string, len(ciphertexts))
	for i, ct := range ciphertexts {
		ctBytes, err := utils.EncodeShare(ct)
		if err != nil {
			return fmt.Errorf("序列化密文 %d 失败: %v", i, err)
		}
		serializedCiphertexts[i] = utils.EncodeToBase64(ctBytes)
	}

	// 构造数据消息
	dataMsg := DataMessage{
		Type:      dataType + "_batch", // "feature_batch" 或 "label_batch"
		From:      mp.participant.ID,
		Data:      utils.EncodeToBase64([]byte(fmt.Sprintf("%d,%d", batchID, 1))), // 批次信息
		BatchData: serializedCiphertexts,
	}

	// 序列化消息
	jsonData, err := json.Marshal(dataMsg)
	if err != nil {
		return fmt.Errorf("序列化数据消息失败: %v", err)
	}

	// 构造请求
	reqBody := map[string]interface{}{
		"from":    mp.participant.ID,
		"message": string(jsonData),
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %v", err)
	}

	// 发送HTTP POST请求
	resp, err := mp.participant.Client.Client.Post(targetURL+"/message", "application/json", bytes.NewReader(reqJSON))
	if err != nil {
		return fmt.Errorf("发送密文数据到参与方 %d 失败: %v", targetID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("参与方 %d 返回错误状态码: %d", targetID, resp.StatusCode)
	}

	return nil
}

// DataMessage 数据消息结构
type DataMessage struct {
	Type      string   `json:"type"` // "feature", "label", "done", "input_done", "output_done"
	From      int      `json:"from"`
	Data      string   `json:"data,omitempty"`       // base64编码的密文
	BatchData []string `json:"batch_data,omitempty"` // base64编码的密文批次
}
