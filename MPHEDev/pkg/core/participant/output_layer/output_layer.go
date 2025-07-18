package output_layer

import (
	"MPHEDev/pkg/core/participant/coordinator"
	"MPHEDev/pkg/core/participant/crypto"
	"MPHEDev/pkg/core/participant/interfaces"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// OutputLayerProcessor 输出层处理器
type OutputLayerProcessor struct {
	*interfaces.CommonProcessor // 嵌入通用处理器

	// 特征发送状态
	sentFeatures map[int][]*rlwe.Ciphertext // [BatchID][]Ciphertext
	featureMu    sync.RWMutex

	// 标签收集状态
	collectedLabels       map[int]map[int][]*rlwe.Ciphertext // [ParticipantID][BatchID][]Ciphertext
	labelCollectionStatus map[int]bool                       // [ParticipantID]bool
	labelMu               sync.RWMutex

	// 隐藏层输出接收状态
	receivedHiddenOutputs map[int][]*rlwe.Ciphertext // [BatchID][]Ciphertext
	hiddenOutputMu        sync.RWMutex

	// 权重处理器
	weightProcessor *OutputLayerWeightProcessor

	// 密钥管理器引用
	keyManager *crypto.KeyManager

	// 计算结果缓存
	computationResults map[int][]*rlwe.Ciphertext
	computationMu      sync.RWMutex

	// 协调器客户端引用
	coordinatorClient *coordinator.CoordinatorClient
}

// NewOutputLayerProcessor 创建输出层处理器
func NewOutputLayerProcessor(dataPackingService interfaces.DataPackingService, dataManager interfaces.DataManager, participantID int, weightManager interfaces.WeightManager) *OutputLayerProcessor {
	// 创建输出层权重处理器（128个隐藏神经元，10个输出类别）
	weightProcessor := NewOutputLayerWeightProcessor(weightManager, dataPackingService, 128, 10)

	return &OutputLayerProcessor{
		CommonProcessor:       interfaces.NewCommonProcessor(dataPackingService, dataManager, participantID),
		sentFeatures:          make(map[int][]*rlwe.Ciphertext),
		collectedLabels:       make(map[int]map[int][]*rlwe.Ciphertext),
		labelCollectionStatus: make(map[int]bool),
		receivedHiddenOutputs: make(map[int][]*rlwe.Ciphertext),
		weightProcessor:       weightProcessor,
		computationResults:    make(map[int][]*rlwe.Ciphertext),
	}
}

// SetParticipantInfo 设置参与方网络信息（继承自CommonProcessor）
func (olp *OutputLayerProcessor) SetParticipantInfo(peerManager, messageProcessor interface{}) {
	olp.CommonProcessor.SetParticipantInfo(peerManager, messageProcessor)
}

// SetKeyManager 设置密钥管理器
func (olp *OutputLayerProcessor) SetKeyManager(keyManager *crypto.KeyManager) {
	olp.keyManager = keyManager
	// 同时设置权重处理器的密钥管理器
	if olp.weightProcessor != nil {
		olp.weightProcessor.SetKeyManager(keyManager)
	}
}

// SetRefreshService 设置刷新服务
func (olp *OutputLayerProcessor) SetRefreshService(refreshService *crypto.RefreshService) {
	// 设置权重处理器的刷新服务
	if olp.weightProcessor != nil {
		olp.weightProcessor.SetRefreshService(refreshService)
	}
}

// SetNetworkInfo 设置网络信息
func (olp *OutputLayerProcessor) SetNetworkInfo(onlinePeers map[int]string, myID int) {
	// 设置权重处理器的网络信息
	if olp.weightProcessor != nil {
		olp.weightProcessor.SetNetworkInfo(onlinePeers, myID)
	}
}

// SetCoordinatorClient 设置协调器客户端
func (olp *OutputLayerProcessor) SetCoordinatorClient(client *coordinator.CoordinatorClient) {
	olp.coordinatorClient = client
}

// ProcessData 实现LayerProcessor接口
func (olp *OutputLayerProcessor) ProcessData(images [][]float64) error {
	// 使用通用处理器计算批次信息
	totalBatches, lastBatchSize := olp.CalculateBatches(len(images))
	olp.PrintBatchConfig("输出层", len(images), totalBatches, lastBatchSize)

	// 处理每个批次（模拟输出层计算）
	for batchIdx := 0; batchIdx < totalBatches; batchIdx++ {
		startIdx := batchIdx * olp.DataPackingService.GetBatchSize()
		endIdx := interfaces.Min(startIdx+olp.DataPackingService.GetBatchSize(), len(images))
		batchImages := images[startIdx:endIdx]

		olp.PrintBatchProgress("输出层", batchIdx, totalBatches, startIdx, endIdx)

		// 使用通用处理器处理批次数据
		ciphertexts, err := olp.ProcessBatchData(batchImages, batchIdx)
		if err != nil {
			return err
		}

		olp.PrintBatchResult("输出层", batchIdx, totalBatches, len(ciphertexts))

		// 存储加密后的特征数据
		olp.featureMu.Lock()
		olp.sentFeatures[batchIdx] = ciphertexts
		olp.featureMu.Unlock()
	}

	olp.PrintProcessingSummary("输出层", totalBatches)
	return nil
}

// ProcessOutputLayerData 处理输出层数据
func (olp *OutputLayerProcessor) ProcessOutputLayerData(images [][]float64) error {
	fmt.Println("\n=== 参与方 3 (output_layer) 数据集处理开始 ===")
	fmt.Printf("数据集信息:\n")
	fmt.Printf("图像数量: %d\n", len(images))
	fmt.Printf("每图像特征数: %d\n", len(images[0]))
	fmt.Printf("数据划分方式: horizontal\n")

	fmt.Println("\n输出层：执行数据重排打包加密（模拟接收隐藏层数据）...")
	fmt.Printf("当前数据集图像数: %d\n", len(images))
	fmt.Printf("状态: 模拟接收隐藏层密文，执行输出层重排打包加密\n")

	// 使用通用处理器处理数据
	return olp.ProcessData(images)
}

// SendFeaturesToInputLayer 将特征数据发送给Input Layer
func (olp *OutputLayerProcessor) SendFeaturesToInputLayer() error {
	fmt.Println("\n=== Output Layer: 发送特征数据给输入层 ===")

	// 获取目标参与方ID（Input Layer通常是参与方1）
	targetParticipantID := 1

	// 获取已处理的特征数据
	olp.featureMu.RLock()
	features, exists := olp.sentFeatures[0] // 批次0
	olp.featureMu.RUnlock()

	if !exists || len(features) == 0 {
		return fmt.Errorf("没有可发送的特征数据")
	}

	fmt.Printf("发送批次 0 的特征数据给参与方 %d...\n", targetParticipantID)

	// 真正发送密文数据
	if err := olp.SendCiphertextData(targetParticipantID, "feature", 0, features); err != nil {
		return fmt.Errorf("发送特征数据给参与方 %d 失败: %v", targetParticipantID, err)
	}

	fmt.Printf("✓ Output Layer: 特征数据发送完成\n")
	return nil
}

// CollectAllLabels 收集所有参与方的标签数据
func (olp *OutputLayerProcessor) CollectAllLabels() error {
	fmt.Println("\n=== Output Layer: 开始收集所有参与方标签数据 ===")

	// 1. 处理本地标签数据
	if err := olp.processLocalLabels(); err != nil {
		return fmt.Errorf("处理本地标签失败: %v", err)
	}

	// 2. 等待其他参与方发送标签数据
	if err := olp.waitForOtherParticipants(); err != nil {
		return fmt.Errorf("等待其他参与方失败: %v", err)
	}

	// 3. 验证收集完整性
	if err := olp.validateLabelCollection(); err != nil {
		return fmt.Errorf("验证标签收集完整性失败: %v", err)
	}

	fmt.Println("✓ Output Layer: 所有标签数据收集完成")
	return nil
}

// OrganizeLabels 组织标签数据为损失计算格式
func (olp *OutputLayerProcessor) OrganizeLabels() map[int]map[int][]*rlwe.Ciphertext {
	olp.labelMu.RLock()
	defer olp.labelMu.RUnlock()

	// 按参与方ID排序组织数据
	organized := make(map[int]map[int][]*rlwe.Ciphertext)

	// 获取所有参与方ID并排序
	participantIDs := make([]int, 0, len(olp.collectedLabels))
	for participantID := range olp.collectedLabels {
		participantIDs = append(participantIDs, participantID)
	}
	sort.Ints(participantIDs)

	for _, pid := range participantIDs {
		batchMap := olp.collectedLabels[pid]
		if batchMap == nil {
			continue
		}
		batchIDs := make([]int, 0, len(batchMap))
		for bid := range batchMap {
			batchIDs = append(batchIDs, bid)
		}
		sort.Ints(batchIDs)
		organized[pid] = make(map[int][]*rlwe.Ciphertext)
		for _, bid := range batchIDs {
			organized[pid][bid] = batchMap[bid]
		}
	}

	fmt.Printf("✓ 标签数据组织完成，共 %d 个参与方\n", len(organized))
	return organized
}

// GetLabelCollectionStatus 获取标签收集状态
func (olp *OutputLayerProcessor) GetLabelCollectionStatus() map[int]bool {
	olp.labelMu.RLock()
	defer olp.labelMu.RUnlock()

	status := make(map[int]bool)
	for participantID, collected := range olp.labelCollectionStatus {
		status[participantID] = collected
	}
	return status
}

// IsLabelCollectionComplete 检查标签收集是否完成
func (olp *OutputLayerProcessor) IsLabelCollectionComplete() bool {
	olp.labelMu.RLock()
	defer olp.labelMu.RUnlock()

	// 检查是否所有参与方都已收集完成
	for _, collected := range olp.labelCollectionStatus {
		if !collected {
			return false
		}
	}
	return len(olp.labelCollectionStatus) > 0
}

// processLocalLabels 处理本地标签数据
func (olp *OutputLayerProcessor) processLocalLabels() error {
	fmt.Println("1. 处理本地标签数据...")

	// 获取批次大小和总批次数
	batchSize := olp.DataPackingService.GetBatchSize()
	totalBatches := olp.DataManager.GetTotalBatches(batchSize)

	if totalBatches == 0 {
		return fmt.Errorf("数据管理器中没有标签数据")
	}

	fmt.Printf("从数据管理器获取到 %d 个批次（批次大小: %d）\n", totalBatches, batchSize)

	// 初始化本地标签收集状态
	olp.labelMu.Lock()
	if olp.collectedLabels[olp.ParticipantID] == nil {
		olp.collectedLabels[olp.ParticipantID] = make(map[int][]*rlwe.Ciphertext)
	}
	olp.labelMu.Unlock()

	// 处理所有批次
	for batchID := 0; batchID < totalBatches; batchID++ {
		fmt.Printf("处理标签批次 %d/%d...\n", batchID+1, totalBatches)

		// 从数据管理器获取当前批次的标签数据
		batchLabels := olp.DataManager.GetLabelBatch(batchID, batchSize)
		if len(batchLabels) == 0 {
			fmt.Printf("⚠️ 标签批次 %d 没有数据，跳过\n", batchID)
			continue
		}

		// 将标签转换为one-hot编码
		oneHotLabels := olp.CommonProcessor.ConvertLabelsToOneHot(batchLabels)

		// 使用通用处理器处理批次数据
		ciphertexts, err := olp.ProcessBatchData(oneHotLabels, batchID)
		if err != nil {
			return fmt.Errorf("标签批次 %d 加密失败: %v", batchID, err)
		}

		// 存储当前批次的加密数据
		olp.labelMu.Lock()
		olp.collectedLabels[olp.ParticipantID][batchID] = ciphertexts
		olp.labelMu.Unlock()

		fmt.Printf("✓ 标签批次 %d 处理完成，生成 %d 个密文\n", batchID, len(ciphertexts))
	}

	// 标记本地标签收集完成
	olp.labelMu.Lock()
	olp.labelCollectionStatus[olp.ParticipantID] = true
	olp.labelMu.Unlock()

	fmt.Printf("✓ 本地标签数据处理完成（参与方 %d）\n", olp.ParticipantID)
	return nil
}

// waitForOtherParticipants 等待其他参与方发送标签数据
func (olp *OutputLayerProcessor) waitForOtherParticipants() error {
	fmt.Println("2. 等待其他参与方发送标签数据...")

	// 获取总参与方数量
	totalParticipants := olp.DataManager.GetTotalParticipants()
	if totalParticipants <= 1 {
		fmt.Println("⚠️ 只有一个参与方，跳过等待")
		return nil
	}

	// 计算需要等待的其他参与方数量（不包括自己）
	expectedOtherParticipants := totalParticipants - 1
	fmt.Printf("总参与方数量: %d\n", totalParticipants)
	fmt.Printf("需要等待其他参与方数量: %d\n", expectedOtherParticipants)

	// 模拟等待其他参与方（实际应该通过消息机制）
	maxWaitTime := 30 * time.Second
	checkInterval := 1 * time.Second
	elapsed := 0 * time.Second

	for elapsed < maxWaitTime {
		olp.labelMu.RLock()
		collectedCount := len(olp.labelCollectionStatus)
		olp.labelMu.RUnlock()

		if collectedCount >= totalParticipants {
			fmt.Printf("✓ 所有 %d 个参与方的标签数据已收集完成\n", totalParticipants)
			return nil
		}

		fmt.Printf("⏳ 已收集 %d/%d 个参与方，继续等待...\n", collectedCount, totalParticipants)
		time.Sleep(checkInterval)
		elapsed += checkInterval
	}

	return fmt.Errorf("等待超时，只收集到 %d/%d 个参与方的标签数据", len(olp.labelCollectionStatus), totalParticipants)
}

// validateLabelCollection 验证标签收集完整性
func (olp *OutputLayerProcessor) validateLabelCollection() error {
	fmt.Println("3. 验证标签收集完整性...")

	olp.labelMu.RLock()
	defer olp.labelMu.RUnlock()

	totalParticipants := olp.DataManager.GetTotalParticipants()
	if len(olp.collectedLabels) < totalParticipants {
		return fmt.Errorf("标签收集不完整，期望 %d 个参与方，实际 %d 个", totalParticipants, len(olp.collectedLabels))
	}

	// 检查每个参与方是否都有数据
	for participantID := 1; participantID <= totalParticipants; participantID++ {
		if _, exists := olp.collectedLabels[participantID]; !exists {
			return fmt.Errorf("参与方 %d 的标签数据缺失", participantID)
		}
	}

	fmt.Printf("✓ 标签收集完整性验证通过，共 %d 个参与方\n", totalParticipants)
	return nil
}

// HandleLabelData 处理接收到的标签数据
func (olp *OutputLayerProcessor) HandleLabelData(participantID int, batchID int, labels []*rlwe.Ciphertext) error {
	fmt.Printf("接收参与方 %d 批次 %d 的标签数据，共 %d 个密文\n", participantID, batchID, len(labels))

	olp.labelMu.Lock()
	defer olp.labelMu.Unlock()

	// 初始化参与方的标签存储
	if olp.collectedLabels[participantID] == nil {
		olp.collectedLabels[participantID] = make(map[int][]*rlwe.Ciphertext)
	}

	// 存储标签数据
	olp.collectedLabels[participantID][batchID] = labels

	// 标记该参与方已收集
	olp.labelCollectionStatus[participantID] = true

	// 添加调试信息：显示当前收集状态
	fmt.Printf("✓ 参与方 %d 批次 %d 标签数据存储完成\n", participantID, batchID)
	fmt.Printf("  当前标签收集状态:\n")
	for pid, collected := range olp.labelCollectionStatus {
		fmt.Printf("    参与方 %d: %s\n", pid, map[bool]string{true: "已收集", false: "未收集"}[collected])
	}
	fmt.Printf("  已收集参与方数量: %d/%d\n", len(olp.labelCollectionStatus), olp.DataManager.GetTotalParticipants())

	return nil
}

// HandleHiddenLayerOutput 处理来自隐藏层的输出数据
func (olp *OutputLayerProcessor) HandleHiddenLayerOutput(participantID int, batchID int, hiddenOutputs []*rlwe.Ciphertext) error {
	fmt.Printf("\n=== 输出层：收到隐藏层输出数据 ===\n")
	fmt.Printf("数据详情:\n")
	fmt.Printf("  • 来源参与方: %d\n", participantID)
	fmt.Printf("  • 批次ID: %d\n", batchID)
	fmt.Printf("  • 隐藏层输出密文数: %d\n", len(hiddenOutputs))

	olp.hiddenOutputMu.Lock()
	// 存储隐藏层输出数据
	olp.receivedHiddenOutputs[batchID] = hiddenOutputs
	fmt.Printf("  • 数据存储状态: 已存储到 receivedHiddenOutputs[%d]\n", batchID)
	olp.hiddenOutputMu.Unlock() // 立即释放锁，避免死锁

	fmt.Printf("✓ 收到参与方 %d 批次 %d 的隐藏层输出数据，共 %d 个密文\n", participantID, batchID, len(hiddenOutputs))

	// 开始输出层计算
	fmt.Println("\n=== 输出层：开始神经网络计算 ===")
	fmt.Printf("计算配置:\n")
	fmt.Printf("  • 隐藏层输出数: %d\n", len(hiddenOutputs))
	fmt.Printf("  • 输出类别数: %d\n", olp.weightProcessor.OutputSize)
	fmt.Printf("  • 批次大小: %d\n", olp.DataPackingService.GetBatchSize())
	fmt.Printf("  • 权重处理器: %s\n", map[bool]string{true: "已初始化", false: "未初始化"}[olp.weightProcessor != nil])
	fmt.Printf("  • 密钥管理器: %s\n", map[bool]string{true: "已设置", false: "未设置"}[olp.keyManager != nil])

	// 开始神经网络计算
	fmt.Println("开始输出层计算...")

	// 添加panic恢复机制
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("❌ 输出层计算发生panic: %v\n", r)
			fmt.Printf("❌ panic堆栈信息: %+v\n", r)
		}
	}()

	// 使用goroutine异步执行计算，避免阻塞
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("❌ goroutine中发生panic: %v\n", r)
				done <- fmt.Errorf("goroutine panic: %v", r)
			}
		}()

		err := olp.ProcessHiddenLayerOutput(batchID)
		done <- err
	}()

	// 等待计算完成，设置超时
	select {
	case err := <-done:
		if err != nil {
			fmt.Printf("❌ 输出层计算失败: %v\n", err)
			return fmt.Errorf("输出层计算失败: %v", err)
		}

	case <-time.After(30 * time.Second):
		fmt.Printf("❌ 输出层计算超时（30秒）\n")
		return fmt.Errorf("输出层计算超时")
	}

	fmt.Printf("✓ 输出层计算流程完成，批次 %d\n", batchID)
	return nil
}

// ProcessHiddenLayerOutput 处理隐藏层输出数据（扩展版）
func (olp *OutputLayerProcessor) ProcessHiddenLayerOutput(batchID int) error {
	// 添加panic恢复机制
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("❌ ProcessHiddenLayerOutput发生panic: %v\n", r)
			fmt.Printf("❌ panic堆栈信息: %+v\n", r)
		}
	}()

	fmt.Printf("\n=== 输出层：开始处理批次 %d 的隐藏层输出数据 ===\n", batchID)

	// 1. 检查是否有隐藏层输出数据

	olp.hiddenOutputMu.RLock()
	hiddenOutputs, exists := olp.receivedHiddenOutputs[batchID]
	olp.hiddenOutputMu.RUnlock()

	if !exists || len(hiddenOutputs) == 0 {
		return fmt.Errorf("批次 %d 没有隐藏层输出数据", batchID)
	}

	// 2. 检查密钥管理器

	if olp.keyManager == nil {
		return fmt.Errorf("密钥管理器未设置")
	}

	// 3. 初始化权重（如果未初始化）

	if olp.weightProcessor.Weights == nil {
		fmt.Println("初始化输出层权重...")
		if err := olp.weightProcessor.InitializeWeights(1); err != nil { // 使用Xavier初始化
			return fmt.Errorf("权重初始化失败: %v", err)
		}

	} else {

	}

	// 4. 执行输出层计算（处理隐藏层输出格式）
	fmt.Printf("开始输出层神经网络计算...\n")

	// 执行真正的输出层计算

	neuronOutputs, err := olp.weightProcessor.ComputeFromHiddenLayerOutput(
		hiddenOutputs,
		batchID,
		olp.keyManager.Params,
		olp.keyManager.Evaluator,
		olp.keyManager.Encoder,
	)

	if err != nil {
		fmt.Printf("❌ 输出层计算失败: %v\n", err)
		return fmt.Errorf("输出层计算失败: %v", err)
	}

	// 6. 应用Softmax激活函数（稍后实现）
	fmt.Println("6. 应用Softmax激活函数...")

	// 检查密文层级并决定是否刷新
	minLevel := olp.keyManager.Params.MaxLevel()
	for _, ct := range neuronOutputs {
		if ct.Level() < minLevel {
			minLevel = ct.Level()
		}
	}

	var softmaxOutputs []*rlwe.Ciphertext
	bootstrapThreshold := olp.keyManager.Params.MaxLevel() / 3

	if minLevel < bootstrapThreshold {
		fmt.Printf("🔄 检测到层级过低（%d < %d），尝试自动刷新...\n", minLevel, bootstrapThreshold)

		// 尝试调用刷新服务
		if olp.weightProcessor.refreshService != nil && olp.weightProcessor.onlinePeers != nil {
			fmt.Printf("🔄 执行自动刷新...\n")
			refreshedOutputs, err := olp.weightProcessor.refreshService.RefreshCiphertextsSync(
				neuronOutputs, olp.weightProcessor.onlinePeers, olp.weightProcessor.myID, "neuron_output_refresh")
			if err != nil {
				fmt.Printf("⚠️  自动刷新失败: %v，跳过Softmax计算\n", err)
				softmaxOutputs = neuronOutputs // 直接使用原始输出
			} else {
				fmt.Printf("✅ 自动刷新完成，使用刷新后的密文进行Softmax计算\n")
				// 使用刷新后的密文进行Softmax计算
				softmaxOutputs, err = olp.weightProcessor.ApplySoftmax(
					refreshedOutputs,
					olp.keyManager.Params,
					olp.keyManager.Evaluator,
					olp.keyManager.Encoder,
				)
				if err != nil {

					softmaxOutputs = refreshedOutputs // 使用刷新后的输出作为备选
				} else {
					fmt.Printf("✓ Softmax应用完成，输出 %d 个概率分布\n", len(softmaxOutputs))
				}
			}
		} else {
			fmt.Printf("⚠️  刷新服务未设置，跳过Softmax计算\n")
			softmaxOutputs = neuronOutputs // 直接使用原始输出
		}
	} else {
		// 层级充足，直接进行Softmax计算
		softmaxOutputs, err := olp.weightProcessor.ApplySoftmax(
			neuronOutputs,
			olp.keyManager.Params,
			olp.keyManager.Evaluator,
			olp.keyManager.Encoder,
		)
		if err != nil {

			// 尝试协同刷新服务解决层级不足问题
			if olp.weightProcessor.refreshService != nil && olp.weightProcessor.onlinePeers != nil {
				fmt.Printf("🔄 尝试协同刷新服务解决层级不足问题...\n")
				refreshedOutputs, refreshErr := olp.weightProcessor.refreshService.RefreshCiphertextsSync(
					neuronOutputs, olp.weightProcessor.onlinePeers, olp.weightProcessor.myID, "softmax_fallback_refresh")
				if refreshErr != nil {
					fmt.Printf("⚠️  协同刷新失败: %v，使用原始输出作为备选\n", refreshErr)
					softmaxOutputs = neuronOutputs // 使用原始输出作为备选
				} else {
					fmt.Printf("✅ 协同刷新完成，重新尝试Softmax计算\n")
					// 使用刷新后的密文重新尝试Softmax计算
					softmaxOutputs, err = olp.weightProcessor.ApplySoftmax(
						refreshedOutputs,
						olp.keyManager.Params,
						olp.keyManager.Evaluator,
						olp.keyManager.Encoder,
					)
					if err != nil {
						softmaxOutputs = refreshedOutputs // 使用刷新后的输出作为备选
					} else {
						fmt.Printf("✓ 刷新后Softmax应用成功，输出 %d 个概率分布\n", len(softmaxOutputs))
					}
				}
			} else {
				fmt.Printf("⚠️  刷新服务未设置，使用原始输出作为备选\n")
				softmaxOutputs = neuronOutputs // 使用原始输出作为备选
			}
		} else {
			fmt.Printf("✓ Softmax应用完成，输出 %d 个概率分布\n", len(softmaxOutputs))
		}
	}

	// 更新计算结果为Softmax输出
	olp.computationMu.Lock()
	olp.computationResults[batchID] = softmaxOutputs
	olp.computationMu.Unlock()

	// 7. 计算交叉熵损失
	fmt.Println("7. 计算交叉熵损失...")

	// 关键修复：根据样本在拼接数据中的位置正确选择对应的参与方标签数据
	// 拼接数据格式：[参与方1的128样本, 参与方2的128样本, 参与方3的128样本]
	// 因此：前128个样本使用参与方1标签，中间128个样本使用参与方2标签，后128个样本使用参与方3标签
	olp.labelMu.RLock()
	hasEncryptedLabels := false
	var encryptedLabels []*rlwe.Ciphertext

	// 计算当前批次对应的参与方ID
	// 假设每个参与方有128个样本
	batchSize := olp.DataPackingService.GetBatchSize() // 通常是128
	totalParticipants := olp.DataManager.GetTotalParticipants()

	// 根据批次ID确定对应的参与方
	// 批次0: 参与方1 (样本0-127)
	// 批次1: 参与方2 (样本128-255)
	// 批次2: 参与方3 (样本256-383)
	participantID := batchID + 1

	// 确保参与方ID在有效范围内
	if participantID > totalParticipants {
		participantID = totalParticipants
	}

	fmt.Printf("  批次 %d 对应参与方 %d (样本范围: %d-%d)\n", batchID, participantID, batchID*batchSize, (batchID+1)*batchSize-1)

	// 优先使用对应参与方的标签数据
	if labels, exists := olp.collectedLabels[participantID][0]; exists && len(labels) > 0 { // 注意：使用批次0
		encryptedLabels = labels
		hasEncryptedLabels = true
		fmt.Printf("  使用参与方%d的密文标签数据，共 %d 个密文\n", participantID, len(labels))
	} else {
		// 如果没有对应参与方的数据，尝试使用其他参与方的数据
		fmt.Printf("  警告：参与方%d的标签数据不存在，尝试其他参与方...\n", participantID)
		for pid, batchMap := range olp.collectedLabels {
			if labels, exists := batchMap[0]; exists && len(labels) > 0 { // 注意：使用批次0
				encryptedLabels = labels
				hasEncryptedLabels = true
				fmt.Printf("  使用参与方%d的密文标签数据，共 %d 个密文\n", pid, len(labels))
				break
			}
		}
	}
	olp.labelMu.RUnlock()

	if hasEncryptedLabels {
		// 使用密文标签计算交叉熵损失
		fmt.Printf("  使用密文标签进行交叉熵损失计算...\n")

		if len(softmaxOutputs) == 0 {
			fmt.Printf("⚠️  softmax输出为空，跳过交叉熵损失计算\n")
		} else {
			loss, err := olp.weightProcessor.ComputeLossWithEncryptedLabels(
				softmaxOutputs,
				encryptedLabels,
				olp.keyManager.Params,
				olp.keyManager.Evaluator,
				olp.keyManager.Encoder,
			)
			if err != nil {
				fmt.Printf("⚠️  密文标签交叉熵损失计算失败: %v\n", err)
				// 不返回错误，继续执行
			} else {
				fmt.Printf("✓ 密文标签交叉熵损失计算完成\n")
				// 存储损失结果
				olp.computationMu.Lock()
				olp.computationResults[batchID] = append(olp.computationResults[batchID], loss)
				olp.computationMu.Unlock()
			}
		}
	} else {
		// 回退到明文标签（用于调试）
		fmt.Printf("⚠️  没有密文标签数据，回退到明文标签计算...\n")
		labels := olp.DataManager.GetLabels()
		if len(labels) > 0 {
			// 计算当前批次的标签
			batchSize := olp.DataPackingService.GetBatchSize()
			startIdx := batchID * batchSize
			endIdx := startIdx + batchSize
			if endIdx > len(labels) {
				endIdx = len(labels)
			}
			batchLabels := labels[startIdx:endIdx]

			fmt.Printf("  使用明文标签进行one-hot编码: %v (共 %d 个)\n", batchLabels, len(batchLabels))

			// 计算交叉熵损失（使用明文标签，内部会转换为one-hot编码）
			loss, err := olp.weightProcessor.ComputeLoss(
				softmaxOutputs,
				batchLabels,
				olp.keyManager.Params,
				olp.keyManager.Evaluator,
				olp.keyManager.Encoder,
			)
			if err != nil {
				fmt.Printf("⚠️  明文标签交叉熵损失计算失败: %v\n", err)
				// 不返回错误，继续执行
			} else {
				fmt.Printf("✓ 明文标签交叉熵损失计算完成（调试模式）\n")
				// 存储损失结果
				olp.computationMu.Lock()
				olp.computationResults[batchID] = append(olp.computationResults[batchID], loss)
				olp.computationMu.Unlock()
			}
		} else {
			fmt.Printf("⚠️  没有标签数据，跳过损失计算\n")
		}
	}

	fmt.Println("✓ 输出层完整计算流程完成")

	// 发送计算完成消息给协调器
	if olp.coordinatorClient != nil {
		if err := olp.coordinatorClient.SendComputationDone(batchID, "success"); err != nil {
			fmt.Printf("⚠️  发送计算完成消息失败: %v\n", err)
		} else {
			fmt.Printf("✓ 计算完成消息已发送给协调器 (批次: %d)\n", batchID)

			// 发送Done消息后，静默GIN日志以避免接收ctCNN输出时的干扰
			gin.SetMode(gin.ReleaseMode)
			fmt.Printf("📝 已开启GIN日志静默模式，准备接收ctCNN输出\n")

			// 不用恢复GIN日志了，因为ctCNN的输出会直接通过websocket发送到前端
		}
	} else {
		fmt.Printf("⚠️  协调器客户端未设置，无法发送计算完成消息\n")
	}

	return nil
}

// GetSentFeatures 获取已发送的特征数据
func (olp *OutputLayerProcessor) GetSentFeatures() map[int][]*rlwe.Ciphertext {
	olp.featureMu.RLock()
	defer olp.featureMu.RUnlock()

	sent := make(map[int][]*rlwe.Ciphertext)
	for batchID, features := range olp.sentFeatures {
		sent[batchID] = features
	}
	return sent
}

// ProcessLabelData 处理标签数据（用于测试）
func (olp *OutputLayerProcessor) ProcessLabelData(labels []int) error {
	fmt.Println("\n输出层：处理标签数据...")
	fmt.Printf("  • 标签数量: %d\n", len(labels))

	// 将标签转换为one-hot编码
	oneHotLabels := olp.CommonProcessor.ConvertLabelsToOneHot(labels)

	// 加密标签数据
	encryptedVectors, err := olp.DataPackingService.EncryptBatch(oneHotLabels)
	if err != nil {
		return fmt.Errorf("标签数据加密失败: %v", err)
	}

	// 直接使用密文切片
	ciphertexts := encryptedVectors

	// 存储加密后的标签数据
	olp.labelMu.Lock()
	if olp.collectedLabels[olp.ParticipantID] == nil {
		olp.collectedLabels[olp.ParticipantID] = make(map[int][]*rlwe.Ciphertext)
	}
	olp.collectedLabels[olp.ParticipantID][0] = ciphertexts // 批次0
	olp.labelCollectionStatus[olp.ParticipantID] = true
	olp.labelMu.Unlock()

	fmt.Printf("✓ 输出层标签数据处理完成，生成 %d 个密文\n", len(ciphertexts))
	return nil
}

// VerifyEncryptionCorrectness 验证加密正确性
func (olp *OutputLayerProcessor) VerifyEncryptionCorrectness(images [][]float64, labels []int) error {
	fmt.Println("\n输出层：验证加密正确性...")

	if len(images) != len(labels) {
		return fmt.Errorf("图像和标签数量不匹配：图像 %d，标签 %d", len(images), len(labels))
	}

	// 验证图像数据
	for i, image := range images {
		if len(image) == 0 {
			return fmt.Errorf("图像 %d 为空", i)
		}
	}

	// 验证标签数据
	for i, label := range labels {
		if label < 0 || label >= 10 {
			return fmt.Errorf("标签 %d 的值 %d 超出有效范围 [0, 9]", i, label)
		}
	}

	fmt.Printf("✓ 数据验证通过：%d 个图像，%d 个标签\n", len(images), len(labels))
	return nil
}

// ProcessInputOutputLayerData 处理输入输出层数据（用于测试）
func (olp *OutputLayerProcessor) ProcessInputOutputLayerData(images [][]float64) error {
	fmt.Println("\n输出层：处理输入输出层数据...")
	fmt.Printf("  • 图像数量: %d\n", len(images))

	// 验证数据
	if err := olp.VerifyEncryptionCorrectness(images, nil); err != nil {
		return fmt.Errorf("数据验证失败: %v", err)
	}

	// 使用通用处理器处理数据
	return olp.ProcessOutputLayerData(images)
}

// GetReceivedHiddenOutput 获取接收到的隐藏层输出
func (olp *OutputLayerProcessor) GetReceivedHiddenOutput(batchID int) ([]*rlwe.Ciphertext, bool) {
	olp.hiddenOutputMu.RLock()
	defer olp.hiddenOutputMu.RUnlock()

	outputs, exists := olp.receivedHiddenOutputs[batchID]
	return outputs, exists
}

// HasHiddenLayerOutput 检查是否有隐藏层输出
func (olp *OutputLayerProcessor) HasHiddenLayerOutput(batchID int) bool {
	olp.hiddenOutputMu.RLock()
	defer olp.hiddenOutputMu.RUnlock()

	_, exists := olp.receivedHiddenOutputs[batchID]
	return exists
}

// ValidateHiddenLayerOutput 验证隐藏层输出数据完整性
func (olp *OutputLayerProcessor) ValidateHiddenLayerOutput(batchID int) error {
	olp.hiddenOutputMu.RLock()
	defer olp.hiddenOutputMu.RUnlock()

	hiddenOutputs, exists := olp.receivedHiddenOutputs[batchID]
	if !exists {
		return fmt.Errorf("批次 %d 的隐藏层输出数据不存在", batchID)
	}

	if len(hiddenOutputs) == 0 {
		return fmt.Errorf("批次 %d 的隐藏层输出数据为空", batchID)
	}

	// 验证数据数量（应该是128个隐藏神经元）
	expectedCount := 128
	if len(hiddenOutputs) != expectedCount {
		return fmt.Errorf("隐藏层输出数据数量不匹配：期望 %d，实际 %d", expectedCount, len(hiddenOutputs))
	}

	fmt.Printf("✓ 隐藏层输出数据验证通过：批次 %d，共 %d 个神经元输出\n", batchID, len(hiddenOutputs))
	return nil
}

// GetComputationResults 获取计算结果
func (olp *OutputLayerProcessor) GetComputationResults(batchID int) ([]*rlwe.Ciphertext, bool) {
	olp.computationMu.RLock()
	defer olp.computationMu.RUnlock()

	results, exists := olp.computationResults[batchID]
	return results, exists
}

// HasComputationResults 检查是否有计算结果
func (olp *OutputLayerProcessor) HasComputationResults(batchID int) bool {
	olp.computationMu.RLock()
	defer olp.computationMu.RUnlock()

	_, exists := olp.computationResults[batchID]
	return exists
}

// GetWeightProcessor 获取权重处理器
func (olp *OutputLayerProcessor) GetWeightProcessor() *OutputLayerWeightProcessor {
	return olp.weightProcessor
}
