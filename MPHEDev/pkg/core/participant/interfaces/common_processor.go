package interfaces

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// CommonProcessor 通用处理器，提供基本的公共功能
type CommonProcessor struct {
	DataPackingService DataPackingService
	DataManager        DataManager
	ParticipantID      int
	PeerManager        interface{}
	MessageProcessor   interface{}

	// 通用状态管理
	Mu sync.RWMutex
}

// NewCommonProcessor 创建通用处理器
func NewCommonProcessor(dataPackingService DataPackingService, dataManager DataManager, participantID int) *CommonProcessor {
	return &CommonProcessor{
		DataPackingService: dataPackingService,
		DataManager:        dataManager,
		ParticipantID:      participantID,
	}
}

// SetParticipantInfo 设置参与方网络信息
func (cp *CommonProcessor) SetParticipantInfo(peerManager, messageProcessor interface{}) {
	cp.PeerManager = peerManager
	cp.MessageProcessor = messageProcessor
}

// CalculateBatches 计算批次信息
func (cp *CommonProcessor) CalculateBatches(totalItems int) (totalBatches, lastBatchSize int) {
	batchSize := cp.DataPackingService.GetBatchSize()
	totalBatches = totalItems / batchSize
	if totalItems%batchSize != 0 {
		totalBatches++
	}
	lastBatchSize = totalItems % batchSize
	if lastBatchSize == 0 {
		lastBatchSize = batchSize
	}
	return totalBatches, lastBatchSize
}

// PrintBatchConfig 打印批次配置信息
func (cp *CommonProcessor) PrintBatchConfig(layerName string, totalItems, totalBatches, lastBatchSize int) {
	fmt.Printf("\n%s批次配置:\n", layerName)
	fmt.Printf("每批图像数: %d\n", cp.DataPackingService.GetBatchSize())
	fmt.Printf("总批次数: %d\n", totalBatches)
	fmt.Printf("最后一批图像数: %d\n", lastBatchSize)
}

// PrintBatchProgress 打印批次处理进度
func (cp *CommonProcessor) PrintBatchProgress(layerName string, batchIdx, totalBatches, startIdx, endIdx int) {
	fmt.Printf("\n%s处理批次 %d/%d (图像 %d-%d)...\n",
		layerName, batchIdx+1, totalBatches, startIdx+1, endIdx)
}

// PrintBatchResult 打印批次处理结果
func (cp *CommonProcessor) PrintBatchResult(layerName string, batchIdx, totalBatches, ciphertextCount int) {
	fmt.Printf("✓ %s批次 %d 加密完成，生成 %d 个密文向量\n",
		layerName, batchIdx+1, ciphertextCount)
}

// PrintProcessingSummary 打印处理总结
func (cp *CommonProcessor) PrintProcessingSummary(layerName string, totalBatches int) {
	fmt.Printf("\n=== %s数据处理完成 ===\n", layerName)
	fmt.Printf("总处理图像数: %d\n", cp.DataPackingService.GetProcessedImages())
	fmt.Printf("总生成密文向量数: %d\n", totalBatches*cp.DataPackingService.GetNumVectors())
	fmt.Printf("平均每图像密文向量数: %.2f\n", float64(cp.DataPackingService.GetNumVectors()))
}

// ConvertToCiphertexts 将密文切片转换为密文切片（保持兼容性）
func (cp *CommonProcessor) ConvertToCiphertexts(encryptedVectors []*rlwe.Ciphertext, batchIdx int) ([]*rlwe.Ciphertext, error) {
	// 现在直接返回密文切片，无需类型转换
	return encryptedVectors, nil
}

// ProcessBatchData 处理单个批次数据
func (cp *CommonProcessor) ProcessBatchData(batchImages [][]float64, batchIdx int) ([]*rlwe.Ciphertext, error) {
	// 重排打包加密
	encryptedVectors, err := cp.DataPackingService.EncryptBatch(batchImages)
	if err != nil {
		return nil, fmt.Errorf("批次 %d 加密失败: %v", batchIdx, err)
	}

	// 类型转换
	ciphertexts, err := cp.ConvertToCiphertexts(encryptedVectors, batchIdx)
	if err != nil {
		return nil, err
	}

	// 更新处理统计
	cp.DataPackingService.AddProcessedImages(len(batchImages))

	return ciphertexts, nil
}

// GetMessageSender 获取消息发送器
func (cp *CommonProcessor) GetMessageSender() (MessageSender, error) {
	messageSender, ok := cp.MessageProcessor.(MessageSender)
	if !ok {
		return nil, fmt.Errorf("消息处理器不支持MessageSender接口")
	}
	return messageSender, nil
}

// SendCiphertextData 发送密文数据
func (cp *CommonProcessor) SendCiphertextData(targetID int, dataType string, batchID int, ciphertexts []*rlwe.Ciphertext) error {
	messageSender, err := cp.GetMessageSender()
	if err != nil {
		return err
	}

	// 发送数据
	return messageSender.SendCiphertextData(targetID, dataType, batchID, ciphertexts)
}

// WaitForCollection 等待收集完成
func (cp *CommonProcessor) WaitForCollection(collectionStatus map[int]bool, timeout time.Duration, collectionType string) error {
	fmt.Printf("  等待其他参与方发送%s数据...\n", collectionType)

	startTime := time.Now()
	for {
		allCollected := true
		for _, collected := range collectionStatus {
			if !collected {
				allCollected = false
				break
			}
		}

		if allCollected {
			fmt.Printf("  ✓ 所有参与方%s数据已收集完成\n", collectionType)
			return nil
		}

		if time.Since(startTime) > timeout {
			return fmt.Errorf("等待%s数据收集超时", collectionType)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// ValidateCollection 验证收集完整性
func (cp *CommonProcessor) ValidateCollection(collectionStatus map[int]bool, collectionType string) error {
	fmt.Printf("  验证%s收集完整性...\n", collectionType)

	for participantID, collected := range collectionStatus {
		if !collected {
			return fmt.Errorf("参与方 %d 的%s数据未收集完成", participantID, collectionType)
		}
	}

	fmt.Printf("  ✓ %s收集完整性验证通过\n", collectionType)
	return nil
}

// OrganizeData 组织数据为有序格式
func (cp *CommonProcessor) OrganizeData(dataMap map[int]map[int][]*rlwe.Ciphertext, dataType string) map[int]map[int][]*rlwe.Ciphertext {
	cp.Mu.RLock()
	defer cp.Mu.RUnlock()

	organized := make(map[int]map[int][]*rlwe.Ciphertext)

	// 获取所有参与方ID并排序
	participantIDs := make([]int, 0, len(dataMap))
	for id := range dataMap {
		participantIDs = append(participantIDs, id)
	}
	sort.Ints(participantIDs)

	for _, pid := range participantIDs {
		batchMap := dataMap[pid]
		if batchMap == nil {
			continue
		}
		// 对批次ID排序
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

	fmt.Printf("✓ %s数据组织完成，共 %d 个参与方\n", dataType, len(organized))
	return organized
}

// GetCollectionStatus 获取收集状态副本
func (cp *CommonProcessor) GetCollectionStatus(collectionStatus map[int]bool) map[int]bool {
	cp.Mu.RLock()
	defer cp.Mu.RUnlock()

	status := make(map[int]bool)
	for participantID, collected := range collectionStatus {
		status[participantID] = collected
	}
	return status
}

// IsCollectionComplete 检查收集是否完成
func (cp *CommonProcessor) IsCollectionComplete(collectionStatus map[int]bool, dataType string) bool {
	for _, collected := range collectionStatus {
		if !collected {
			return false
		}
	}
	return len(collectionStatus) > 0
}

// ProcessLocalLabels 处理本地标签数据（与MNIST_CNN保持一致）
// MNIST_CNN的标签处理：标签不重排，按批次存储，转换为one-hot编码后加密
func (cp *CommonProcessor) ProcessLocalLabels() ([]*rlwe.Ciphertext, error) {
	fmt.Println("1. 处理本地标签数据（与MNIST_CNN保持一致）...")

	// 获取批次大小和总批次数
	batchSize := cp.DataPackingService.GetBatchSize()
	totalBatches := cp.DataManager.GetTotalBatches(batchSize)

	if totalBatches == 0 {
		return nil, fmt.Errorf("数据管理器中没有标签数据")
	}

	fmt.Printf("从数据管理器获取到 %d 个批次（批次大小: %d）\n", totalBatches, batchSize)

	// 处理所有批次
	for batchID := 0; batchID < totalBatches; batchID++ {
		fmt.Printf("处理标签批次 %d/%d...\n", batchID+1, totalBatches)

		// 从数据管理器获取当前批次的标签数据
		batchLabels := cp.DataManager.GetLabelBatch(batchID, batchSize)
		if len(batchLabels) == 0 {
			fmt.Printf("⚠️ 批次 %d 没有标签数据，跳过\n", batchID)
			continue
		}

		// 使用与MNIST_CNN一致的标签处理方法
		// 标签不重排，按批次存储，转换为one-hot编码后加密
		ciphertexts, err := cp.DataPackingService.ProcessLabelsWithPacking(batchLabels)
		if err != nil {
			return nil, fmt.Errorf("批次 %d 标签数据处理失败: %v", batchID, err)
		}

		fmt.Printf("✓ 标签批次 %d 处理完成，生成 %d 个密文\n", batchID, len(ciphertexts))
		return ciphertexts, nil // 只处理第一个批次
	}

	fmt.Println("✓ 本地标签数据处理完成")
	return nil, fmt.Errorf("没有可处理的标签数据")
}

// ConvertLabelsToOneHot 将标签转换为one-hot编码（通用方法）
func (cp *CommonProcessor) ConvertLabelsToOneHot(labels []int) [][]float64 {
	numClasses := 10 // MNIST数据集有10个类别
	oneHot := make([][]float64, len(labels))

	for i, label := range labels {
		oneHot[i] = make([]float64, numClasses)
		if label >= 0 && label < numClasses {
			oneHot[i][label] = 1.0
		}
	}

	return oneHot
}
