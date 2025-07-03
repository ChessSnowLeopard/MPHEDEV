package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"MPHEDev/pkg/core/participant/utils"
)

// SendMessageToParticipant 向指定参与方发送消息
func (p *Participant) SendMessageToParticipant(participantID int, message string) error {
	var peerURL string
	var exists bool

	// 如果是向自己发送消息，使用自己的URL
	if participantID == p.ID {
		peerURL = fmt.Sprintf("http://%s:%d", p.HTTPServer.GetLocalIP(), p.Port)
		exists = true
	} else {
		// 获取目标参与方的URL
		peerURL, exists = p.PeerManager.GetPeerURL(participantID)
	}

	if !exists {
		return fmt.Errorf("参与方 %d 不在线或不存在", participantID)
	}

	// 构造请求体
	reqBody := map[string]interface{}{
		"from":    p.ID,
		"message": message,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	// 发送HTTP请求
	resp, err := p.Client.Client.Post(peerURL+"/message", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("发送消息失败: %d", resp.StatusCode)
	}

	fmt.Printf("消息已发送到参与方 %d\n", participantID)
	return nil
}

// SendMessageToAll 向所有在线参与方发送消息
func (p *Participant) SendMessageToAll(message string) error {
	peers := p.PeerManager.GetPeers()

	// 如果没有其他参与方，至少向自己发送消息
	if len(peers) == 0 {
		return p.SendMessageToParticipant(p.ID, message)
	}

	// 向其他参与方发送
	for id := range peers {
		if err := p.SendMessageToParticipant(id, message); err != nil {
			fmt.Printf("向参与方 %d 发送消息失败: %v\n", id, err)
		}
	}

	// 向自己发送
	if err := p.SendMessageToParticipant(p.ID, message); err != nil {
		fmt.Printf("向自己发送消息失败: %v\n", err)
	}

	return nil
}

// HandleMessage 处理来自其他参与方的消息
func (p *Participant) HandleMessage(fromID int, message string) {
	var dataMsg DataMessage
	if err := json.Unmarshal([]byte(message), &dataMsg); err != nil {
		fmt.Printf("解析消息失败: %v\n", err)
		return
	}
	fmt.Printf("收到消息：类型=%s，来自=%d\n", dataMsg.Type, fromID)
	switch dataMsg.Type {
	case "feature":
		p.handleFeatureData(fromID, dataMsg)
	case "label":
		p.handleLabelData(fromID, dataMsg)
	case "feature_batch":
		p.handleFeatureBatchData(fromID, dataMsg)
	case "label_batch":
		p.handleLabelBatchData(fromID, dataMsg)
	case "done":
		p.handleDoneMessage(fromID, dataMsg)
	case "input_done":
		p.handleInputDoneMessage(fromID, dataMsg)
	case "output_done":
		p.handleOutputDoneMessage(fromID, dataMsg)
	default:
		fmt.Printf("未知消息类型: %s\n", dataMsg.Type)
	}
}

// handleFeatureData 处理特征数据
func (p *Participant) handleFeatureData(fromID int, msg DataMessage) {
	fmt.Printf("参与方 %d 收到来自参与方 %d 的特征数据\n", p.ID, fromID)

	// 标记已接收该参与方的特征数据
	p.ReceivedFeatures[fromID] = true

	// 检查是否所有参与方的特征数据都已接收
	p.checkDataDistributionStatus()
}

// handleLabelData 处理标签数据
func (p *Participant) handleLabelData(fromID int, msg DataMessage) {
	fmt.Printf("参与方 %d 收到来自参与方 %d 的标签数据\n", p.ID, fromID)

	// 标记已接收该参与方的标签数据
	p.ReceivedLabels[fromID] = true

	// 检查是否所有参与方的标签数据都已接收
	p.checkDataDistributionStatus()
}

// handleFeatureBatchData 处理特征数据批次
func (p *Participant) handleFeatureBatchData(fromID int, msg DataMessage) {
	// 解析批次信息
	batchInfoBytes, err := utils.DecodeFromBase64(msg.Data)
	if err != nil {
		fmt.Printf("解析特征批次信息失败: %v\n", err)
		return
	}

	var currentBatch, totalBatches int
	fmt.Sscanf(string(batchInfoBytes), "%d,%d", &currentBatch, &totalBatches)

	fmt.Printf("参与方 %d 收到来自参与方 %d 的特征数据批次 %d/%d (包含 %d 个密文)\n",
		p.ID, fromID, currentBatch, totalBatches, len(msg.BatchData))

	// 初始化批次状态
	if p.FeatureBatchStatus[fromID] == nil {
		p.FeatureBatchStatus[fromID] = &BatchStatus{
			TotalBatches:    totalBatches,
			ReceivedBatches: make(map[int]bool),
			AllReceived:     false,
		}
	}

	// 标记当前批次已接收
	p.FeatureBatchStatus[fromID].ReceivedBatches[currentBatch] = true

	// 存储接收到的密文数据
	if p.ReceivedFeatureCiphertexts[fromID] == nil {
		p.ReceivedFeatureCiphertexts[fromID] = make([][]string, totalBatches)
	}
	p.ReceivedFeatureCiphertexts[fromID][currentBatch-1] = msg.BatchData // 批次索引从0开始

	// 检查是否所有批次都已接收
	allBatchesReceived := true
	for i := 1; i <= totalBatches; i++ {
		if !p.FeatureBatchStatus[fromID].ReceivedBatches[i] {
			allBatchesReceived = false
			break
		}
	}

	if allBatchesReceived {
		fmt.Printf("参与方 %d 已接收来自参与方 %d 的所有特征数据批次\n", p.ID, fromID)
		p.FeatureBatchStatus[fromID].AllReceived = true
		p.ReceivedFeatures[fromID] = true

		// 检查是否所有参与方的特征数据都已接收
		p.checkDataDistributionStatus()
	}
}

// handleLabelBatchData 处理标签数据批次
func (p *Participant) handleLabelBatchData(fromID int, msg DataMessage) {
	// 解析批次信息
	batchInfoBytes, err := utils.DecodeFromBase64(msg.Data)
	if err != nil {
		fmt.Printf("解析标签批次信息失败: %v\n", err)
		return
	}

	var currentBatch, totalBatches int
	fmt.Sscanf(string(batchInfoBytes), "%d,%d", &currentBatch, &totalBatches)

	fmt.Printf("参与方 %d 收到来自参与方 %d 的标签数据批次 %d/%d (包含 %d 个密文)\n",
		p.ID, fromID, currentBatch, totalBatches, len(msg.BatchData))

	// 初始化批次状态
	if p.LabelBatchStatus[fromID] == nil {
		p.LabelBatchStatus[fromID] = &BatchStatus{
			TotalBatches:    totalBatches,
			ReceivedBatches: make(map[int]bool),
			AllReceived:     false,
		}
	}

	// 标记当前批次已接收
	p.LabelBatchStatus[fromID].ReceivedBatches[currentBatch] = true

	// 存储接收到的密文数据
	if p.ReceivedLabelCiphertexts[fromID] == nil {
		p.ReceivedLabelCiphertexts[fromID] = make([][]string, totalBatches)
	}
	p.ReceivedLabelCiphertexts[fromID][currentBatch-1] = msg.BatchData // 批次索引从0开始

	// 检查是否所有批次都已接收
	allBatchesReceived := true
	for i := 1; i <= totalBatches; i++ {
		if !p.LabelBatchStatus[fromID].ReceivedBatches[i] {
			allBatchesReceived = false
			break
		}
	}

	if allBatchesReceived {
		fmt.Printf("参与方 %d 已接收来自参与方 %d 的所有标签数据批次\n", p.ID, fromID)
		p.LabelBatchStatus[fromID].AllReceived = true
		p.ReceivedLabels[fromID] = true

		// 检查是否所有参与方的标签数据都已接收
		p.checkDataDistributionStatus()
	}
}

// handleDoneMessage 处理完成消息
func (p *Participant) handleDoneMessage(fromID int, msg DataMessage) {
	fmt.Printf("参与方 %d 收到来自参与方 %d 的完成消息\n", p.ID, fromID)

	// 标记数据分发完成
	p.DataDistributionDone = true

	// 通知主线程数据分发已完成
	select {
	case <-p.ReadyCh:
		// 通道已关闭，不需要操作
	default:
		close(p.ReadyCh)
	}
}

// handleInputDoneMessage 处理输入层完成消息
func (p *Participant) handleInputDoneMessage(fromID int, msg DataMessage) {
	fmt.Printf("参与方 %d 收到来自参与方 %d 的输入层完成消息\n", p.ID, fromID)
	p.InputLayerDone = true
	p.checkDataDistributionStatus()
}

// handleOutputDoneMessage 处理输出层完成消息
func (p *Participant) handleOutputDoneMessage(fromID int, msg DataMessage) {
	fmt.Printf("参与方 %d 收到来自参与方 %d 的输出层完成消息\n", p.ID, fromID)
	p.OutputLayerDone = true
	p.checkDataDistributionStatus()
}

// checkDataDistributionStatus 检查数据分发状态
func (p *Participant) checkDataDistributionStatus() {
	// 获取所有参与方
	allParticipants := p.PeerManager.GetPeers()
	allParticipants[p.ID] = "" // 添加自己

	// 检查是否所有参与方的特征和标签数据都已接收
	allFeaturesReceived := true
	allLabelsReceived := true

	for participantID := range allParticipants {
		if !p.ReceivedFeatures[participantID] {
			allFeaturesReceived = false
		}
		if !p.ReceivedLabels[participantID] {
			allLabelsReceived = false
		}
	}

	// 如果所有数据都已接收，发送相应的Done消息
	if allFeaturesReceived && allLabelsReceived && !p.DataDistributionDone {
		fmt.Printf("参与方 %d 已接收所有数据，发送Done消息\n", p.ID)
		p.DataDistributionDone = true

		// 根据参与方角色发送相应的Done消息
		// 发送输入层Done消息
		inputDoneMessage := DataMessage{
			Type: "input_done",
			From: p.ID,
		}
		inputDoneJSON, err := json.Marshal(inputDoneMessage)
		if err != nil {
			fmt.Printf("序列化输入层Done消息失败: %v\n", err)
		} else {
			if err := p.SendMessageToAll(string(inputDoneJSON)); err != nil {
				fmt.Printf("发送输入层Done消息失败: %v\n", err)
			}
		}

		// 发送输出层Done消息
		outputDoneMessage := DataMessage{
			Type: "output_done",
			From: p.ID,
		}
		outputDoneJSON, err := json.Marshal(outputDoneMessage)
		if err != nil {
			fmt.Printf("序列化输出层Done消息失败: %v\n", err)
		} else {
			if err := p.SendMessageToAll(string(outputDoneJSON)); err != nil {
				fmt.Printf("发送输出层Done消息失败: %v\n", err)
			}
		}

		// 通知主线程数据分发已完成
		select {
		case <-p.ReadyCh:
			// 通道已关闭，不需要操作
		default:
			close(p.ReadyCh)
		}
	}
}

// testP2PCommunication 测试P2P通信功能
func (p *Participant) testP2PCommunication() error {
	// 获取在线参与方列表
	onlineParticipants := p.GetOnlineParticipants()
	if len(onlineParticipants) == 0 {
		fmt.Printf("没有其他在线参与方，跳过P2P通信测试\n")
		return nil
	}

	fmt.Printf("开始P2P通信测试，在线参与方: %d 个\n", len(onlineParticipants))

	// 向所有参与方发送测试消息
	testMessage := fmt.Sprintf("来自参与方 %d 的P2P通信测试消息", p.ID)
	if err := p.SendMessageToAll(testMessage); err != nil {
		return fmt.Errorf("P2P通信测试失败: %v", err)
	}

	fmt.Printf("P2P通信测试成功\n")
	return nil
}
