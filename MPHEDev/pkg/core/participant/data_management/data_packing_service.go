package data_management

import (
	"MPHEDev/pkg/core/participant/interfaces"
	"fmt"
	"math"
	"math/rand"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// DataPackingService 数据集重排打包加密服务
type DataPackingService struct {
	// 基础配置参数
	SlotCount     int // CKKS slot数 (s)
	FeatureCount  int // 每图像特征数 (k)
	ImageFeatures int // 图像特征总数 (t=784)
	BatchSize     int // 批次大小 (s/k)
	NumVectors    int // 向量总数 (ceil(t/k))

	// 加密相关
	Params    ckks.Parameters
	Encoder   *ckks.Encoder
	Encryptor *rlwe.Encryptor

	// 数据管理
	CurrentBatch      int
	TotalBatches      int
	ProcessedImages   int
	featuresPerCipher int  // 每密文特征数
	EncryptionEnabled bool // 是否启用加密功能
}

// NewDataPackingService 创建新的数据打包服务
func NewDataPackingService(slotCount, featureCount int, params ckks.Parameters, encryptor *rlwe.Encryptor) *DataPackingService {
	// 计算派生参数
	imageFeatures := 784 // MNIST图像特征数
	batchSize := slotCount / featureCount
	numVectors := (imageFeatures + featureCount - 1) / featureCount // 向上取整

	// 创建编码器（如果参数有效）
	var encoder *ckks.Encoder
	if params.LogN() > 0 {
		encoder = ckks.NewEncoder(params)
	}

	return &DataPackingService{
		SlotCount:         slotCount,
		FeatureCount:      featureCount,
		ImageFeatures:     imageFeatures,
		BatchSize:         batchSize,
		NumVectors:        numVectors,
		Params:            params,
		Encoder:           encoder,
		Encryptor:         encryptor,
		CurrentBatch:      0,
		TotalBatches:      0,
		ProcessedImages:   0,
		featuresPerCipher: 64,   // 默认每密文特征数
		EncryptionEnabled: true, // 默认启用加密
	}
}

// GetBatchSize 获取批次大小
func (dps *DataPackingService) GetBatchSize() int {
	return dps.BatchSize
}

// GetNumVectors 获取向量总数
func (dps *DataPackingService) GetNumVectors() int {
	return dps.NumVectors
}

// GetSlotCount 获取CKKS slot数
func (dps *DataPackingService) GetSlotCount() int {
	return dps.SlotCount
}

// GetParams 获取CKKS参数
func (dps *DataPackingService) GetParams() ckks.Parameters {
	return dps.Params
}

// GetEncryptor 获取加密器
func (dps *DataPackingService) GetEncryptor() *rlwe.Encryptor {
	return dps.Encryptor
}

// GetEncoder 获取编码器
func (dps *DataPackingService) GetEncoder() *ckks.Encoder {
	return dps.Encoder
}

// GetProcessedImages 获取已处理图像数
func (dps *DataPackingService) GetProcessedImages() int {
	return dps.ProcessedImages
}

// AddProcessedImages 增加已处理图像数
func (dps *DataPackingService) AddProcessedImages(count int) {
	dps.ProcessedImages += count
}

// PrintConfiguration 打印配置信息
func (dps *DataPackingService) PrintConfiguration() {
	fmt.Printf("\n=== 数据打包配置 ===\n")
	fmt.Printf("CKKS slot数: %d\n", dps.SlotCount)
	fmt.Printf("每图像特征数: %d\n", dps.FeatureCount)
	fmt.Printf("图像特征总数: %d\n", dps.ImageFeatures)
	fmt.Printf("批次大小: %d\n", dps.BatchSize)
	fmt.Printf("向量总数: %d\n", dps.NumVectors)
	if dps.ImageFeatures%dps.FeatureCount != 0 {
		fmt.Printf("最后向量特征数: %d/%d\n", dps.ImageFeatures%dps.FeatureCount, dps.FeatureCount)
	}
	fmt.Printf("重排模式: 按特征分组打包\n")
	fmt.Printf("====================\n")
}

// AssembleBatch 将一批图像组装成重排向量（完全遵循MNIST_CNN逻辑）
// 核心重排逻辑：将多张图像的相同位置像素值放在一起
func (dps *DataPackingService) AssembleBatch(images [][]float64) ([][]complex128, error) {
	// 如果图像数量不足，进行填充
	if len(images) < dps.BatchSize {
		fmt.Printf("  注意: 图像数量(%d)少于批次大小(%d)，进行零填充\n", len(images), dps.BatchSize)

		// 创建填充后的图像数组
		paddedImages := make([][]float64, dps.BatchSize)

		// 复制原始图像
		for i := 0; i < len(images); i++ {
			paddedImages[i] = make([]float64, len(images[i]))
			copy(paddedImages[i], images[i])
		}

		// 用零向量填充剩余位置
		for i := len(images); i < dps.BatchSize; i++ {
			paddedImages[i] = make([]float64, dps.ImageFeatures)
			// 所有元素默认为0
		}

		images = paddedImages
	}

	// 初始化结果向量数组
	vectors := make([][]complex128, dps.NumVectors)
	for i := range vectors {
		vectors[i] = make([]complex128, dps.SlotCount)
	}

	fmt.Printf("✓ 开始特征重排打包，完全遵循MNIST_CNN逻辑\n")
	fmt.Printf("  图像数量: %d, 批次大小: %d, 特征数: %d\n", len(images), dps.BatchSize, dps.ImageFeatures)
	fmt.Printf("  向量数量: %d, 槽数: %d, 每向量特征数: %d\n", dps.NumVectors, dps.SlotCount, dps.FeatureCount)

	// 组装向量：每个向量处理k个连续的特征位置
	// 完全遵循MNIST_CNN的AssembleVectors逻辑：外层特征，内层样本
	for vectorIdx := 0; vectorIdx < dps.NumVectors; vectorIdx++ {
		// 当前向量处理的特征范围
		featureStart := vectorIdx * dps.FeatureCount
		featureEnd := interfaces.Min(featureStart+dps.FeatureCount, dps.ImageFeatures)

		// 修正数据打包顺序：外层特征，内层样本（与MNIST_CNN完全一致）
		for featureOffset := 0; featureOffset < featureEnd-featureStart; featureOffset++ {
			pixelIndex := featureStart + featureOffset
			for imageIdx := 0; imageIdx < dps.BatchSize; imageIdx++ {
				vectorPosition := featureOffset*dps.BatchSize + imageIdx
				if vectorPosition < dps.SlotCount && imageIdx < len(images) && pixelIndex < len(images[imageIdx]) {
					// 数据归一化：0-255 → 0-1（与MNIST_CNN一致）
					pixelValue := images[imageIdx][pixelIndex] / 255.0
					vectors[vectorIdx][vectorPosition] = complex(pixelValue, 0)
				}
			}
		}
		// 向量剩余位置自动保持为0（默认值）
	}

	fmt.Printf("✓ 特征重排完成，生成 %d 个向量\n", dps.NumVectors)
	fmt.Printf("  重排格式: [样本0特征0, 样本1特征0, ..., 样本n-1特征0 | 样本0特征1, 样本1特征1, ...]\n")

	return vectors, nil
}

// ProcessLabelsWithPacking 处理标签数据（完全遵循MNIST_CNN逻辑）
// MNIST_CNN的标签处理：标签按照样本顺序重复打包，确保与特征数据的样本顺序完全一致
func (dps *DataPackingService) ProcessLabelsWithPacking(labels []int) ([]*rlwe.Ciphertext, error) {
	fmt.Println("1. 处理标签数据（完全遵循MNIST_CNN逻辑）...")

	// 获取批次大小和总批次数
	batchSize := dps.BatchSize
	totalBatches := (len(labels) + batchSize - 1) / batchSize // 向上取整

	if totalBatches == 0 {
		return nil, fmt.Errorf("没有可处理的标签数据")
	}

	fmt.Printf("标签数据信息:\n")
	fmt.Printf("  • 标签数量: %d\n", len(labels))
	fmt.Printf("  • 批次大小: %d\n", batchSize)
	fmt.Printf("  • 总批次数: %d\n", totalBatches)

	// 处理所有批次
	for batchID := 0; batchID < totalBatches; batchID++ {
		fmt.Printf("处理标签批次 %d/%d...\n", batchID+1, totalBatches)

		// 计算当前批次的标签范围
		startIdx := batchID * batchSize
		endIdx := startIdx + batchSize
		if endIdx > len(labels) {
			endIdx = len(labels)
		}

		// 提取当前批次的标签（与MNIST_CNN保持一致）
		batchLabels := labels[startIdx:endIdx]

		// 将标签转换为one-hot编码（明文，不加密）
		oneHotLabels := dps.ConvertLabelsToOneHot(batchLabels)

		// 将one-hot标签编码为明文向量（不加密）
		labelVectors, err := dps.EncodeOneHotLabels(oneHotLabels)
		if err != nil {
			return nil, fmt.Errorf("批次 %d 标签编码失败: %v", batchID, err)
		}

		// 加密标签向量
		encryptedLabels, err := dps.EncryptVectors(labelVectors)
		if err != nil {
			return nil, fmt.Errorf("批次 %d 标签加密失败: %v", batchID, err)
		}

		// 转换为密文切片
		ciphertexts := make([]*rlwe.Ciphertext, 0, len(encryptedLabels))
		for _, ciphertext := range encryptedLabels {
			ciphertexts = append(ciphertexts, ciphertext)
		}

		fmt.Printf("✓ 标签批次 %d 处理完成，生成 %d 个密文\n", batchID, len(ciphertexts))
		return ciphertexts, nil // 只处理第一个批次
	}

	fmt.Println("✓ 标签数据处理完成")
	return nil, fmt.Errorf("没有可处理的标签数据")
}

// ConvertLabelsToOneHot 将标签转换为one-hot编码（与MNIST_CNN保持一致）
func (dps *DataPackingService) ConvertLabelsToOneHot(labels []int) [][]float64 {
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

// EncodeOneHotLabels 将one-hot标签编码为明文向量（完全遵循MNIST_CNN逻辑）
func (dps *DataPackingService) EncodeOneHotLabels(oneHotLabels [][]float64) ([][]complex128, error) {
	numClasses := len(oneHotLabels[0]) // 类别数
	vectors := make([][]complex128, numClasses)

	for classIdx := 0; classIdx < numClasses; classIdx++ {
		vectors[classIdx] = make([]complex128, dps.SlotCount)

		// 完全遵循MNIST_CNN的createOneHotLabels逻辑
		// 在每个重复段中填入one-hot值，确保与特征数据的样本顺序完全一致
		for segment := 0; segment < dps.SlotCount/dps.BatchSize; segment++ {
			startIdx := segment * dps.BatchSize
			for sampleIdx := 0; sampleIdx < dps.BatchSize && sampleIdx < len(oneHotLabels); sampleIdx++ {
			if sampleIdx < len(oneHotLabels) && classIdx < len(oneHotLabels[sampleIdx]) {
					// 如果当前样本属于当前类别，设置为1，否则为0
					if oneHotLabels[sampleIdx][classIdx] == 1.0 {
						vectors[classIdx][startIdx+sampleIdx] = complex(1.0, 0)
					} else {
						vectors[classIdx][startIdx+sampleIdx] = complex(0.0, 0)
					}
				} else {
					// 超出范围的样本设置为0
					vectors[classIdx][startIdx+sampleIdx] = complex(0.0, 0)
				}
			}
		}
		// 剩余位置保持为0
	}

	fmt.Printf("✓ 标签编码完成，遵循MNIST_CNN重复打包逻辑\n")
	fmt.Printf("  类别数: %d, 批大小: %d, 槽数: %d\n", numClasses, dps.BatchSize, dps.SlotCount)
	fmt.Printf("  重复段数: %d, 每段样本数: %d\n", dps.SlotCount/dps.BatchSize, dps.BatchSize)

	return vectors, nil
}

// ==================== 权重打包方法 ====================

// PackWeightsForNeuron 为单个神经元打包权重（适用于CKKS）
// neuronWeights: 神经元的权重向量
// start: 起始特征索引
// actualK: 实际处理的特征数（可能小于FeatureCount）
// n: 每个特征段内的样本数
// s: 总槽数
func (dps *DataPackingService) PackWeightsForNeuron(neuronWeights []float64, start, actualK, n, s int) []complex128 {
	packed := make([]complex128, s)

	// 按照公式 W_i^{packed}[j·n + m] = W_{i, start+j}
	for j := 0; j < actualK; j++ {
		if start+j < len(neuronWeights) {
			weight := neuronWeights[start+j]
			// 每个权重复制n次
			for m := 0; m < n; m++ {
				idx := j*n + m
				if idx < s {
					packed[idx] = complex(weight, 0)
				}
			}
		}
	}

	return packed
}

// PackBiasForNeuron 为单个神经元打包偏置项
// bias: 神经元的偏置值
func (dps *DataPackingService) PackBiasForNeuron(bias float64) []complex128 {
	packed := make([]complex128, dps.SlotCount)

	// 偏置项在前n个位置重复，其余位置为0
	for i := 0; i < dps.BatchSize && i < dps.SlotCount; i++ {
		packed[i] = complex(bias, 0)
	}

	return packed
}

// ==================== 工具方法 ====================

// EncryptVectors 加密重排后的向量
func (dps *DataPackingService) EncryptVectors(vectors [][]complex128) (map[uint64]*rlwe.Ciphertext, error) {
	if dps.Encryptor == nil {
		return nil, fmt.Errorf("加密器未初始化")
	}
	if dps.Encoder == nil {
		return nil, fmt.Errorf("编码器未初始化")
	}

	fmt.Printf("开始加密 %d 个向量...\n", len(vectors))
	encryptedVectors := make(map[uint64]*rlwe.Ciphertext)

	for idx, vector := range vectors {
		// 确保向量长度与槽位数匹配
		complexVector := make([]complex128, dps.SlotCount)
		for i := 0; i < len(vector) && i < dps.SlotCount; i++ {
			complexVector[i] = vector[i]
		}

		// 编码
		pt := ckks.NewPlaintext(dps.Params, dps.Params.MaxLevel())
		if err := dps.Encoder.Encode(complexVector, pt); err != nil {
			return nil, fmt.Errorf("编码向量 %d 失败: %v", idx, err)
		}

		// 加密
		ct, err := dps.Encryptor.EncryptNew(pt)
		if err != nil {
			return nil, fmt.Errorf("加密向量 %d 失败: %v", idx, err)
		}

		encryptedVectors[uint64(idx)] = ct
	}

	fmt.Printf("✓ 向量加密完成，共加密 %d 个向量\n", len(encryptedVectors))
	return encryptedVectors, nil
}

// EncryptBatch 加密一个批次的数据
func (dps *DataPackingService) EncryptBatch(batchImages [][]float64) ([]*rlwe.Ciphertext, error) {
	if !dps.EncryptionEnabled {
		return nil, fmt.Errorf("加密功能未启用")
	}

	fmt.Printf("开始加密 %d 个向量...\n", dps.NumVectors)

	// 1. 重排数据
	vectors, err := dps.AssembleBatch(batchImages)
	if err != nil {
		return nil, fmt.Errorf("数据重排失败: %v", err)
	}

	// 2. 加密向量
	encryptedVectors, err := dps.EncryptVectors(vectors)
	if err != nil {
		return nil, fmt.Errorf("向量加密失败: %v", err)
	}

	// 转换为密文切片
	result := make([]*rlwe.Ciphertext, len(encryptedVectors))
	i := 0
	for _, ct := range encryptedVectors {
		result[i] = ct
		i++
	}

	fmt.Printf("✓ 向量加密完成，共加密 %d 个向量\n", dps.NumVectors)
	return result, nil
}

// ProcessDataset 处理整个数据集
func (dps *DataPackingService) ProcessDataset(images [][]float64) error {
	if len(images) == 0 {
		return fmt.Errorf("数据集为空")
	}

	// 计算总批次数
	dps.TotalBatches = len(images) / dps.BatchSize
	if len(images)%dps.BatchSize != 0 {
		dps.TotalBatches++ // 向上取整
	}

	fmt.Printf("数据集总图像数: %d\n", len(images))
	fmt.Printf("每批处理图像数: %d\n", dps.BatchSize)
	fmt.Printf("总批次数: %d\n", dps.TotalBatches)
	fmt.Printf("每批产生向量数: %d\n", dps.NumVectors)

	// 分批处理
	for batchIdx := 0; batchIdx < dps.TotalBatches; batchIdx++ {
		// 计算当前批次的图像范围
		startIdx := batchIdx * dps.BatchSize
		endIdx := interfaces.Min(startIdx+dps.BatchSize, len(images))

		// 提取当前批次的图像
		batchImages := images[startIdx:endIdx]

		// 如果最后一批图像数量不足，需要填充
		if len(batchImages) < dps.BatchSize {
			// 用零向量填充
			padding := make([][]float64, dps.BatchSize-len(batchImages))
			for i := range padding {
				padding[i] = make([]float64, dps.ImageFeatures)
			}
			batchImages = append(batchImages, padding...)
		}

		// 组装当前批次的向量
		vectors, err := dps.AssembleBatch(batchImages)
		if err != nil {
			return fmt.Errorf("组装第%d批次失败: %v", batchIdx, err)
		}

		// 暂时跳过加密步骤
		fmt.Printf("批次 %d: 已组装 %d 个向量（跳过加密）\n", batchIdx+1, len(vectors))

		// 更新进度
		dps.CurrentBatch = batchIdx + 1
		dps.ProcessedImages += len(batchImages)

		// 打印进度
		if (batchIdx+1)%10 == 0 || batchIdx == dps.TotalBatches-1 {
			fmt.Printf("已处理 %d/%d 个批次\n", batchIdx+1, dps.TotalBatches)
		}
	}

	fmt.Printf("✓ 数据集处理完成，总处理图像数: %d\n", dps.ProcessedImages)
	return nil
}

// CreateWeightMatrix 创建并初始化权重矩阵
func (dps *DataPackingService) CreateWeightMatrix(hiddenSize, inputSize int) [][]float64 {
	return dps.CreateWeightMatrixWithMethod(hiddenSize, inputSize, 2) // 默认使用He初始化
}

// CreateWeightMatrixWithMethod 创建并初始化权重矩阵（可选择初始化方法）
// hiddenSize: 隐藏层神经元数量
// inputSize: 输入特征数量
// initMethod: 初始化方法 (1=Xavier, 2=He, 3=标准正态, 4=均匀分布, 5=小方差正态)
func (dps *DataPackingService) CreateWeightMatrixWithMethod(hiddenSize, inputSize, initMethod int) [][]float64 {
	W := make([][]float64, hiddenSize)

	switch initMethod {
	case 1: // Xavier/Glorot初始化 (适用于Sigmoid/Tanh)
		var fanIn, fanOut float64 = float64(inputSize), float64(hiddenSize)
		var stddev float64 = math.Sqrt(2.0 / (fanIn + fanOut))
		for i := 0; i < hiddenSize; i++ {
			W[i] = make([]float64, inputSize)
			for j := 0; j < inputSize; j++ {
				W[i][j] = rand.NormFloat64() * stddev
			}
		}

	case 2: // He初始化 (适用于ReLU)
		var fanIn float64 = float64(inputSize)
		var stddev float64 = math.Sqrt(2.0 / fanIn)
		for i := 0; i < hiddenSize; i++ {
			W[i] = make([]float64, inputSize)
			for j := 0; j < inputSize; j++ {
				W[i][j] = rand.NormFloat64() * stddev
			}
		}

	case 3: // 标准正态分布 (均值0，方差1)
		for i := 0; i < hiddenSize; i++ {
			W[i] = make([]float64, inputSize)
			for j := 0; j < inputSize; j++ {
				W[i][j] = rand.NormFloat64()
			}
		}

	case 4: // 均匀分布初始化
		var fanIn, fanOut float64 = float64(inputSize), float64(hiddenSize)
		var limit float64 = math.Sqrt(6.0 / (fanIn + fanOut))
		for i := 0; i < hiddenSize; i++ {
			W[i] = make([]float64, inputSize)
			for j := 0; j < inputSize; j++ {
				// 均匀分布 [-limit, limit]
				W[i][j] = (rand.Float64()*2.0 - 1.0) * limit
			}
		}

	case 5: // 较小方差的正态分布
		var stddev float64 = 0.01 // 很小的标准差
		for i := 0; i < hiddenSize; i++ {
			W[i] = make([]float64, inputSize)
			for j := 0; j < inputSize; j++ {
				W[i][j] = rand.NormFloat64() * stddev
			}
		}

	default: // Xavier初始化
		var fanIn, fanOut float64 = float64(inputSize), float64(hiddenSize)
		var stddev float64 = math.Sqrt(2.0 / (fanIn + fanOut))
		for i := 0; i < hiddenSize; i++ {
			W[i] = make([]float64, inputSize)
			for j := 0; j < inputSize; j++ {
				W[i][j] = rand.NormFloat64() * stddev
			}
		}
	}

	return W
}

// CreateBiasVector 创建并初始化偏置向量
func (dps *DataPackingService) CreateBiasVector(hiddenSize int) []float64 {
	biases := make([]float64, hiddenSize)

	// 偏置项通常初始化为0或小的随机值
	for i := 0; i < hiddenSize; i++ {
		biases[i] = 0.0 // 初始化为0
		// 或者使用小的随机值：biases[i] = (rand.Float64() - 0.5) * 0.01
	}

	return biases
}

// GetProgress 获取处理进度
func (dps *DataPackingService) GetProgress() (int, int, int) {
	return dps.CurrentBatch, dps.TotalBatches, dps.ProcessedImages
}

// GetFeaturesPerCipher 返回每密文特征数
func (dps *DataPackingService) GetFeaturesPerCipher() int {
	return dps.featuresPerCipher
}

// GetFeatureCount 返回每图像特征数
func (dps *DataPackingService) GetFeatureCount() int {
	return dps.FeatureCount
}

// 使用共享的min函数
