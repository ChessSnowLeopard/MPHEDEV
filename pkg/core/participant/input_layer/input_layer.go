package input_layer

import (
	"MPHEDev/pkg/core/participant/crypto"
	"MPHEDev/pkg/core/participant/interfaces"
	syncservice "MPHEDev/pkg/core/participant/sync"
	"MPHEDev/pkg/core/participant/types"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// InputLayerProcessor 输入层处理器
type InputLayerProcessor struct {
	*interfaces.CommonProcessor // 嵌入通用处理器

	// 特征收集状态
	collectedFeatures map[int]map[int][]*rlwe.Ciphertext // [ParticipantID][BatchID][]Ciphertext
	collectionStatus  map[int]bool                       // [ParticipantID]bool
	featureMu         sync.RWMutex

	// 标签发送状态
	sentLabels map[int][]*rlwe.Ciphertext // [BatchID][]Ciphertext
	labelMu    sync.RWMutex

	// 传输服务
	transferService *TransferService

	// 密钥管理器引用
	keyManager *crypto.KeyManager
}

// NewInputLayerProcessor 创建输入层处理器
func NewInputLayerProcessor(dataPackingService interfaces.DataPackingService, dataManager interfaces.DataManager, participantID int) *InputLayerProcessor {
	return &InputLayerProcessor{
		CommonProcessor:   interfaces.NewCommonProcessor(dataPackingService, dataManager, participantID),
		collectedFeatures: make(map[int]map[int][]*rlwe.Ciphertext),
		collectionStatus:  make(map[int]bool),
		sentLabels:        make(map[int][]*rlwe.Ciphertext),
	}
}

// SetParticipantInfo 设置参与方网络信息（继承自CommonProcessor）
func (ilp *InputLayerProcessor) SetParticipantInfo(peerManager, messageProcessor interface{}) {
	ilp.CommonProcessor.SetParticipantInfo(peerManager, messageProcessor)

	// 初始化传输服务
	if peerManager != nil && messageProcessor != nil {
		// 从messageProcessor获取HTTP客户端
		if mp, ok := messageProcessor.(interface{ GetClient() *http.Client }); ok {
			client := mp.GetClient()
			ilp.transferService = NewTransferService(ilp.ParticipantID, peerManager, client)
		} else {
			// 如果无法获取客户端，使用默认客户端
			ilp.transferService = NewTransferService(ilp.ParticipantID, peerManager, &http.Client{})
		}
	}

	// 动态初始化所有参与方的收集状态
	ilp.featureMu.Lock()

	// 从数据管理器获取总参与方数量
	totalParticipants := ilp.DataManager.GetTotalParticipants()
	if totalParticipants == 0 {
		// 如果数据管理器中没有设置，尝试从peerManager获取
		if pm, ok := peerManager.(interface{ GetPeers() map[int]string }); ok {
			peers := pm.GetPeers()
			totalParticipants = len(peers) + 1 // 包括自己
		} else {
			// 默认使用3个参与方（向后兼容）
			totalParticipants = 3
			fmt.Printf("警告：无法获取参与方数量，使用默认值 %d\n", totalParticipants)
		}
	}

	// 初始化所有参与方的收集状态为false
	for i := 1; i <= totalParticipants; i++ {
		if i != ilp.ParticipantID { // 除了自己，其他参与方初始化为false
			ilp.collectionStatus[i] = false
		}
	}
	ilp.featureMu.Unlock()

	// 构建参与方列表用于显示
	participantList := make([]int, 0, totalParticipants-1)
	for i := 1; i <= totalParticipants; i++ {
		if i != ilp.ParticipantID {
			participantList = append(participantList, i)
		}
	}

	fmt.Printf("初始化收集状态：参与方 %d 等待参与方 %v 的特征数据（共 %d 个参与方）\n",
		ilp.ParticipantID, participantList, totalParticipants)
}

// SetKeyManager 设置密钥管理器
func (ilp *InputLayerProcessor) SetKeyManager(keyManager *crypto.KeyManager) {
	ilp.keyManager = keyManager
}

// getSyncService 获取同步服务
func (ilp *InputLayerProcessor) getSyncService() syncservice.SyncService {
	// 通过messageProcessor获取Participant的同步服务
	if ilp.MessageProcessor != nil {
		if participant, ok := ilp.MessageProcessor.(interface {
			GetSyncService() syncservice.SyncService
		}); ok {
			return participant.GetSyncService()
		}
	}
	return nil
}

// ProcessData 实现LayerProcessor接口
func (ilp *InputLayerProcessor) ProcessData(images [][]float64) error {
	// 使用通用处理器计算批次信息
	totalBatches, lastBatchSize := ilp.CalculateBatches(len(images))
	ilp.PrintBatchConfig("输入层", len(images), totalBatches, lastBatchSize)

	// 处理每个批次
	for batchIdx := 0; batchIdx < totalBatches; batchIdx++ {
		startIdx := batchIdx * ilp.DataPackingService.GetBatchSize()
		endIdx := interfaces.Min(startIdx+ilp.DataPackingService.GetBatchSize(), len(images))
		batchImages := images[startIdx:endIdx]

		ilp.PrintBatchProgress("输入层", batchIdx, totalBatches, startIdx, endIdx)

		// 使用通用处理器处理批次数据
		ciphertexts, err := ilp.ProcessBatchData(batchImages, batchIdx)
		if err != nil {
			return err
		}

		ilp.PrintBatchResult("输入层", batchIdx, totalBatches, len(ciphertexts))

		// 将加密后的数据存储到收集的特征中
		ilp.featureMu.Lock()
		// 初始化参与方的批次映射
		if ilp.collectedFeatures[ilp.ParticipantID] == nil {
			ilp.collectedFeatures[ilp.ParticipantID] = make(map[int][]*rlwe.Ciphertext)
		}
		// 存储加密后的特征数据
		ilp.collectedFeatures[ilp.ParticipantID][batchIdx] = ciphertexts
		ilp.featureMu.Unlock()
	}

	ilp.PrintProcessingSummary("输入层", totalBatches)
	return nil
}

// ProcessInputLayerData 处理输入层数据
func (ilp *InputLayerProcessor) ProcessInputLayerData(images [][]float64) error {
	fmt.Println("\n=== 参与方 1 (input_layer) 数据集处理开始 ===")
	fmt.Printf("数据集信息:\n")
	fmt.Printf("图像数量: %d\n", len(images))
	fmt.Printf("每图像特征数: %d\n", len(images[0]))
	fmt.Printf("数据划分方式: horizontal\n")

	fmt.Println("\n输入层：执行数据重排打包加密...")

	// 使用通用处理器处理数据
	return ilp.ProcessData(images)
}

// CollectAllFeatures 收集所有参与方的特征数据
func (ilp *InputLayerProcessor) CollectAllFeatures() error {
	fmt.Println("\n=== Input Layer: 开始收集所有参与方特征数据 ===")

	// 1. 处理本地特征数据
	if err := ilp.processLocalFeatures(); err != nil {
		return fmt.Errorf("处理本地特征失败: %v", err)
	}

	// 2. 等待其他参与方发送特征数据
	if err := ilp.WaitForCollection(ilp.collectionStatus, 30*time.Second, "特征"); err != nil {
		return fmt.Errorf("等待其他参与方失败: %v", err)
	}

	// 3. 验证收集完整性
	if err := ilp.ValidateCollection(ilp.collectionStatus, "特征"); err != nil {
		return fmt.Errorf("验证收集完整性失败: %v", err)
	}

	fmt.Println("✓ Input Layer: 所有特征数据收集完成")
	return nil
}

// OrganizeFeatures 组织特征数据为神经网络输入格式
func (ilp *InputLayerProcessor) OrganizeFeatures() map[int]map[int][]*rlwe.Ciphertext {
	return ilp.OrganizeData(ilp.collectedFeatures, "特征")
}

// GetCollectionStatus 获取收集状态
func (ilp *InputLayerProcessor) GetCollectionStatus() map[int]bool {
	ilp.featureMu.RLock()
	defer ilp.featureMu.RUnlock()

	status := make(map[int]bool)
	for participantID, collected := range ilp.collectionStatus {
		status[participantID] = collected
	}
	return status
}

// GetFeatureCollectionStatus 获取特征收集状态（接口兼容）
func (ilp *InputLayerProcessor) GetFeatureCollectionStatus() map[int]bool {
	return ilp.GetCollectionStatus()
}

// IsCollectionComplete 检查收集是否完成
func (ilp *InputLayerProcessor) IsCollectionComplete() bool {
	return ilp.CommonProcessor.IsCollectionComplete(ilp.collectionStatus, "特征")
}

// IsFeatureCollectionComplete 检查特征收集是否完成（接口兼容）
func (ilp *InputLayerProcessor) IsFeatureCollectionComplete() bool {
	return ilp.IsCollectionComplete()
}

// processLocalFeatures 处理本地特征数据
func (ilp *InputLayerProcessor) processLocalFeatures() error {
	fmt.Println("1. 处理本地特征数据...")

	// 获取批次大小和总批次数
	batchSize := ilp.DataPackingService.GetBatchSize()
	totalBatches := ilp.DataManager.GetTotalBatches(batchSize)

	if totalBatches == 0 {
		return fmt.Errorf("数据管理器中没有图像数据")
	}

	fmt.Printf("从数据管理器获取到 %d 个批次（批次大小: %d）\n", totalBatches, batchSize)

	// 处理所有批次
	for batchID := 0; batchID < totalBatches; batchID++ {
		fmt.Printf("处理批次 %d/%d...\n", batchID+1, totalBatches)

		// 从数据管理器获取当前批次的图像数据
		batchImages := ilp.DataManager.GetImageBatch(batchID, batchSize)
		if len(batchImages) == 0 {
			fmt.Printf("⚠️ 批次 %d 没有图像数据，跳过\n", batchID)
			continue
		}

		// 使用通用处理器处理批次数据
		ciphertexts, err := ilp.ProcessBatchData(batchImages, batchID)
		if err != nil {
			return fmt.Errorf("批次 %d 特征数据加密失败: %v", batchID, err)
		}

		// 存储当前批次的加密数据
		ilp.featureMu.Lock()
		if ilp.collectedFeatures[ilp.ParticipantID] == nil {
			ilp.collectedFeatures[ilp.ParticipantID] = make(map[int][]*rlwe.Ciphertext)
		}
		ilp.collectedFeatures[ilp.ParticipantID][batchID] = ciphertexts
		ilp.featureMu.Unlock()

		fmt.Printf("✓ 批次 %d 处理完成，生成 %d 个密文\n", batchID, len(ciphertexts))
	}

	// 标记本地特征数据已处理
	ilp.featureMu.Lock()
	ilp.collectionStatus[ilp.ParticipantID] = true
	ilp.featureMu.Unlock()

	fmt.Println("✓ 本地特征数据处理完成")
	return nil
}

// waitForOtherParticipants 等待其他参与方发送数据
func (ilp *InputLayerProcessor) waitForOtherParticipants() error {
	fmt.Println("2. 等待其他参与方发送特征数据...")

	startTime := time.Now()
	timeout := 30 * time.Second

	for {
		allCollected := true
		ilp.featureMu.RLock()
		for _, collected := range ilp.collectionStatus {
			if !collected {
				allCollected = false
				break
			}
		}
		ilp.featureMu.RUnlock()

		if allCollected {
			fmt.Println("✓ 所有参与方特征数据已收集完成")
			return nil
		}

		if time.Since(startTime) > timeout {
			return fmt.Errorf("等待特征数据收集超时")
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// validateCollection 验证收集完整性
func (ilp *InputLayerProcessor) validateCollection() error {
	fmt.Println("3. 验证特征收集完整性...")

	ilp.featureMu.RLock()
	defer ilp.featureMu.RUnlock()

	for participantID, collected := range ilp.collectionStatus {
		if !collected {
			return fmt.Errorf("参与方 %d 的特征数据未收集完成", participantID)
		}
	}

	fmt.Println("✓ 特征收集完整性验证通过")
	return nil
}

// HandleFeatureData 处理接收到的特征数据
func (ilp *InputLayerProcessor) HandleFeatureData(participantID int, batchID int, features []*rlwe.Ciphertext) error {
	ilp.featureMu.Lock()
	defer ilp.featureMu.Unlock()

	// 初始化参与方的批次映射
	if ilp.collectedFeatures[participantID] == nil {
		ilp.collectedFeatures[participantID] = make(map[int][]*rlwe.Ciphertext)
	}

	// 存储特征数据
	ilp.collectedFeatures[participantID][batchID] = features

	// 简化处理：假设收到数据就认为该参与方收集完成
	ilp.collectionStatus[participantID] = true

	fmt.Printf("✓ 收到参与方 %d 批次 %d 的特征数据，共 %d 个密文\n", participantID, batchID, len(features))
	return nil
}

// SendFeaturesToInputLayer 输入层不需要发送特征数据，只需收集
func (ilp *InputLayerProcessor) SendFeaturesToInputLayer() error {
	fmt.Println("\n=== Input Layer: 输入层不发送特征数据，仅收集 ===")
	fmt.Println("✓ Input Layer: 特征数据收集模式")
	return nil
}

// SendFeaturesToHiddenLayer 将特征数据发送给Hidden Layer
func (ilp *InputLayerProcessor) SendFeaturesToHiddenLayer(batchID int, targetID int) error {
	fmt.Printf("\n=== Input Layer: 发送拼接后的特征数据给Hidden Layer (参与方 %d) ===\n", targetID)

	// 拼接所有参与方的特征数据
	var allFeatures []*rlwe.Ciphertext
	ilp.featureMu.RLock()

	// 调试：打印收集到的所有数据
	fmt.Printf("调试：collectedFeatures 内容:\n")
	for pid, batchMap := range ilp.collectedFeatures {
		fmt.Printf("  参与方 %d: ", pid)
		if batchMap == nil {
			fmt.Printf("nil\n")
		} else {
			for bid, features := range batchMap {
				fmt.Printf("批次 %d -> %d 个密文, ", bid, len(features))
			}
			fmt.Printf("\n")
		}
	}

	// 按参与方ID排序，确保拼接顺序一致
	participantIDs := make([]int, 0, len(ilp.collectedFeatures))
	for pid := range ilp.collectedFeatures {
		participantIDs = append(participantIDs, pid)
	}
	sort.Ints(participantIDs)

	fmt.Printf("调试：排序后的参与方ID: %v\n", participantIDs)

	// 拼接每个参与方的特征数据
	for _, pid := range participantIDs {
		if features, exists := ilp.collectedFeatures[pid][batchID]; exists {
			allFeatures = append(allFeatures, features...)
			fmt.Printf("  拼接参与方 %d 的特征数据，共 %d 个密文\n", pid, len(features))
		} else {
			fmt.Printf("  警告：参与方 %d 批次 %d 的数据不存在\n", pid, batchID)
		}
	}
	ilp.featureMu.RUnlock()

	if len(allFeatures) == 0 {
		return fmt.Errorf("批次 %d 没有可拼接的特征数据", batchID)
	}

	fmt.Printf("拼接完成，总共 %d 个密文\n", len(allFeatures))

	// 发送拼接后的特征数据
	if err := ilp.SendCiphertextData(targetID, "feature", batchID, allFeatures); err != nil {
		return fmt.Errorf("发送特征数据失败: %v", err)
	}

	// 等待隐藏层的Done确认
	syncService := ilp.getSyncService()
	if syncService != nil {
		fmt.Printf("等待参与方 %d 的Done确认...\n", targetID)
		if err := syncService.WaitForDoneConfirmation(types.SyncPhaseFeatureTransfer, batchID, targetID, 30*time.Second); err != nil {
			fmt.Printf("警告: 等待Done确认超时: %v\n", err)
		} else {
			fmt.Printf("✓ 收到参与方 %d 的Done确认\n", targetID)
		}
	}

	fmt.Printf("✓ Input Layer: 拼接后的特征数据发送完成\n")
	return nil
}

// GetSendStatus 获取发送状态
func (ilp *InputLayerProcessor) GetSendStatus(batchID int) SendStatus {
	if ilp.transferService == nil {
		return SendStatus{
			BatchID:   batchID,
			Status:    SendStatusPending,
			Timestamp: time.Now(),
		}
	}
	return ilp.transferService.GetSendStatus(batchID)
}

// RetrySend 重试发送
func (ilp *InputLayerProcessor) RetrySend(batchID int) error {
	if ilp.transferService == nil {
		return fmt.Errorf("传输服务未初始化")
	}

	// 获取要发送的特征数据
	ilp.featureMu.RLock()
	features, exists := ilp.collectedFeatures[ilp.ParticipantID][batchID]
	ilp.featureMu.RUnlock()

	if !exists {
		return fmt.Errorf("批次 %d 的特征数据不存在", batchID)
	}

	// 获取目标ID（从状态中获取）
	status := ilp.transferService.GetSendStatus(batchID)
	if status.TargetID == 0 {
		return fmt.Errorf("无法获取目标参与方ID")
	}

	// 使用传输服务重试发送
	return ilp.transferService.RetrySend(batchID, status.TargetID, features)
}

// GetSentFeatures 获取已发送的特征数据
func (ilp *InputLayerProcessor) GetSentFeatures() map[int][]*rlwe.Ciphertext {
	ilp.featureMu.RLock()
	defer ilp.featureMu.RUnlock()

	sent := make(map[int][]*rlwe.Ciphertext)
	for batchID, features := range ilp.collectedFeatures[ilp.ParticipantID] {
		sent[batchID] = features
	}
	return sent
}

// SendLabelsToOutputLayer 将标签数据发送给Output Layer
func (ilp *InputLayerProcessor) SendLabelsToOutputLayer() error {
	fmt.Println("\n=== Input Layer: 开始发送标签数据给Output Layer ===")

	// 获取目标参与方ID（Output Layer通常是参与方3）
	targetParticipantID := 3

	// 处理本地标签数据
	labels, err := ilp.ProcessLocalLabels()
	if err != nil {
		return fmt.Errorf("处理本地标签失败: %v", err)
	}

	if len(labels) == 0 {
		return fmt.Errorf("没有可发送的标签数据")
	}

	fmt.Printf("发送标签数据给参与方 %d...\n", targetParticipantID)

	// 真正发送密文数据
	if err := ilp.SendCiphertextData(targetParticipantID, "label", 0, labels); err != nil {
		return fmt.Errorf("发送标签数据给参与方 %d 失败: %v", targetParticipantID, err)
	}

	// 存储已发送的标签数据
	ilp.labelMu.Lock()
	ilp.sentLabels[0] = labels // 批次0
	ilp.labelMu.Unlock()

	fmt.Printf("✓ Input Layer: 标签数据发送完成\n")
	return nil
}

// GetSentLabels 获取已发送的标签数据
func (ilp *InputLayerProcessor) GetSentLabels() map[int][]*rlwe.Ciphertext {
	ilp.labelMu.RLock()
	defer ilp.labelMu.RUnlock()

	sent := make(map[int][]*rlwe.Ciphertext)
	for batchID, labels := range ilp.sentLabels {
		sent[batchID] = labels
	}
	return sent
}
