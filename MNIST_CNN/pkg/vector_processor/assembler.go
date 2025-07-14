package vector_processor

import (
	"MNIST-CNN/pkg/training"
	"fmt"
	"math"
)

// VectorAssembler 向量组装器
// 用于将多张图像重新组织成适合同态加密的向量格式
type VectorAssembler struct {
	S               int // 向量大小（CKKS插槽数）
	K               int // 每个图像取的特征数
	ImageFeatures   int // 图像特征总数（如MNIST为784）
	ImagesPerVector int // 每个向量容纳的图像数量 (s/k)
	NumVectors      int // 向量总数 (ImageFeatures/k，向上取整)
}

// NewVectorAssembler 创建向量组装器
// s: 向量大小，k: 每个图像的特征数，imageFeatures: 图像特征总数
func NewVectorAssembler(s, k int, imageFeatures int) *VectorAssembler {
	// 计算向量总数：向上取整(imageFeatures/k)
	// 即每个batch拥有多少个向量
	numVectors := (imageFeatures + k - 1) / k

	// 计算每个向量容纳的图像数：向下取整(s/k)
	imagesPerVector := s / k

	return &VectorAssembler{
		S:               s,
		K:               k,
		ImageFeatures:   imageFeatures,
		ImagesPerVector: imagesPerVector,
		NumVectors:      numVectors,
	}
}

// NewVectorAssemblerFromDataset 从数据集自动创建向量组装器
// 自动从数据集中获取图像特征数
func NewVectorAssemblerFromDataset(s, k int, dataset *training.Dataset) *VectorAssembler {
	if len(dataset.Images) == 0 {
		panic("数据集为空，无法计算图像特征数")
	}

	// 从第一张图像获取特征数
	imageFeatures := len(dataset.Images[0])
	return NewVectorAssembler(s, k, imageFeatures)
}

// AssembleVectors 将一批图像组装成向量
// 核心重排逻辑：将多张图像的相同位置像素值放在一起
func (va *VectorAssembler) AssembleVectors(images [][]byte) ([][]float32, error) {
	// 检查图像数量是否足够
	if len(images) < va.ImagesPerVector {
		return nil, fmt.Errorf("图像数量(%d)少于每个向量需要的图像数(%d)",
			len(images), va.ImagesPerVector)
	}

	// 初始化结果向量数组
	vectors := make([][]float32, va.NumVectors)
	for i := range vectors {
		vectors[i] = make([]float32, va.S)
	}

	// 组装向量：每个向量处理k个连续的特征位置
	for vectorIdx := 0; vectorIdx < va.NumVectors; vectorIdx++ {
		// 当前向量处理的特征范围
		featureStart := vectorIdx * va.K
		featureEnd := min(featureStart+va.K, va.ImageFeatures)

		// 修正数据打包顺序：外层特征，内层样本
		for featureOffset := 0; featureOffset < featureEnd-featureStart; featureOffset++ {
			pixelIndex := featureStart + featureOffset
			for imageIdx := 0; imageIdx < va.ImagesPerVector; imageIdx++ {
				vectorPosition := featureOffset*va.ImagesPerVector + imageIdx
				if vectorPosition < va.S && imageIdx < len(images) {
					vectors[vectorIdx][vectorPosition] = float32(images[imageIdx][pixelIndex]) / 255.0
				}
			}
		}
		// 向量剩余位置自动保持为0（默认值）
	}

	return vectors, nil
}

// AssembleDataset 组装整个数据集
// 将数据集分批处理，每批处理ImagesPerVector张图像
func (va *VectorAssembler) AssembleDataset(dataset *training.Dataset) ([][]float32, [][]byte, error) {
	// 计算完整批次数（向下取整）
	numBatches := len(dataset.Images) / va.ImagesPerVector

	fmt.Printf("数据集总图像数: %d\n", len(dataset.Images))
	fmt.Printf("每批处理图像数: %d\n", va.ImagesPerVector)
	fmt.Printf("完整批次数: %d\n", numBatches)
	fmt.Printf("每批产生向量数: %d\n", va.NumVectors)

	// 预分配结果容器
	allVectors := make([][]float32, 0, numBatches*va.NumVectors)
	allLabels := make([][]byte, 0, numBatches)

	// 分批处理图像
	for batchIdx := 0; batchIdx < numBatches; batchIdx++ {
		// 计算当前批次的图像范围
		startIdx := batchIdx * va.ImagesPerVector
		endIdx := startIdx + va.ImagesPerVector

		// 提取当前批次的图像和标签
		batchImages := dataset.Images[startIdx:endIdx]
		batchLabels := dataset.Labels[startIdx:endIdx]

		// 组装当前批次的向量
		vectors, err := va.AssembleVectors(batchImages)
		if err != nil {
			return nil, nil, fmt.Errorf("组装第%d批次失败: %v", batchIdx, err)
		}

		// 添加到总结果中
		allVectors = append(allVectors, vectors...)
		allLabels = append(allLabels, batchLabels)

		// 打印进度
		if (batchIdx+1)%100 == 0 || batchIdx == numBatches-1 {
			fmt.Printf("已处理 %d/%d 个批次\n", batchIdx+1, numBatches)
		}
	}

	fmt.Printf("✓ 数据集组装完成，总向量数: %d\n", len(allVectors))
	return allVectors, allLabels, nil
}

// ComputePlainResult 使用重排后的明文向量计算结果（用于验证）
func (va *VectorAssembler) ComputePlainResult(plainVectors [][]float32, neuronWeights []float64,
	startVecIdx, sampleIdx int) float64 {

	result := 0.0
	inputSize := len(neuronWeights)

	// 遍历所有特征
	for featIdx := 0; featIdx < inputSize; featIdx++ {
		// 找到该特征所在的向量
		vecIdx := featIdx / va.K
		featOffset := featIdx % va.K

		// 全局向量索引
		globalVecIdx := startVecIdx + vecIdx

		// 在向量中的位置
		pixelPos := featOffset*va.ImagesPerVector + sampleIdx

		// 获取像素值并计算
		if globalVecIdx < len(plainVectors) && pixelPos < len(plainVectors[globalVecIdx]) {
			pixelValue := float64(plainVectors[globalVecIdx][pixelPos])
			result += pixelValue * neuronWeights[featIdx]
		}
	}

	return result
}

// ComputeOriginalResult 使用原始数据格式计算结果（用于验证）
func (va *VectorAssembler) ComputeOriginalResult(dataset *training.Dataset, neuronWeights []float64,
	batchIdx, sampleIdx int) float64 {

	// 计算该样本在原始数据集中的全局索引
	globalSampleIdx := batchIdx*va.ImagesPerVector + sampleIdx

	if globalSampleIdx >= len(dataset.Images) {
		return 0.0
	}

	result := 0.0
	image := dataset.Images[globalSampleIdx]

	// 标准矩阵乘法
	for featIdx := 0; featIdx < len(image) && featIdx < len(neuronWeights); featIdx++ {
		pixelValue := float64(image[featIdx]) / 255.0 // 归一化
		result += pixelValue * neuronWeights[featIdx]
	}

	return result
}

// VerifyPackedComputation 验证打包计算的详细过程
func (va *VectorAssembler) VerifyPackedComputation(plainVectors [][]float32, neuronWeights []float64,
	startVecIdx, sampleIdx int, dataset *training.Dataset, batchIdx int) {

	k := va.K
	n := va.ImagesPerVector

	fmt.Printf("\n  重排计算过程分解（k=%d, n=%d）:\n", k, n)

	totalFromPacked := 0.0

	// 遍历每个向量（每轮）
	for round := 0; round < va.NumVectors && round < 5; round++ { // 只显示前5轮
		featureStart := round * k
		featureEnd := min(featureStart+k, va.ImageFeatures)

		roundSum := 0.0
		fmt.Printf("\n  轮 %d (特征 %d-%d):\n", round, featureStart, featureEnd-1)

		for j := 0; j < featureEnd-featureStart; j++ {
			featIdx := featureStart + j

			// 从重排向量中获取值
			globalVecIdx := startVecIdx + round
			pixelPos := j*n + sampleIdx

			if globalVecIdx < len(plainVectors) && pixelPos < len(plainVectors[globalVecIdx]) {
				pixelValue := float64(plainVectors[globalVecIdx][pixelPos])
				weight := neuronWeights[featIdx]
				contribution := pixelValue * weight
				roundSum += contribution

				if j < 3 { // 只显示前3个特征
					fmt.Printf("    特征%d: %.4f × %.4f = %.6f\n",
						featIdx, pixelValue, weight, contribution)
				}
			}
		}

		fmt.Printf("    本轮小计: %.6f\n", roundSum)
		totalFromPacked += roundSum
	}

	// 原始计算验证
	globalSampleIdx := batchIdx*n + sampleIdx
	originalImage := dataset.Images[globalSampleIdx]
	totalOriginal := 0.0

	fmt.Printf("\n  原始计算验证:\n")
	for i := 0; i < min(10, len(originalImage)); i++ {
		pixelValue := float64(originalImage[i]) / 255.0
		contribution := pixelValue * neuronWeights[i]
		totalOriginal += contribution
		if i < 3 {
			fmt.Printf("    特征%d: %.4f × %.4f = %.6f\n",
				i, pixelValue, neuronWeights[i], contribution)
		}
	}

	// 计算完整结果
	for i := 10; i < len(originalImage) && i < len(neuronWeights); i++ {
		pixelValue := float64(originalImage[i]) / 255.0
		totalOriginal += pixelValue * neuronWeights[i]
	}

	fmt.Printf("\n  重排计算总和: %.6f\n", totalFromPacked)
	fmt.Printf("  原始计算总和: %.6f\n", totalOriginal)
	fmt.Printf("  差异: %.9f\n", math.Abs(totalFromPacked-totalOriginal))
}

// PrintVectorInfo 打印向量组装器的配置信息
func (va *VectorAssembler) PrintVectorInfo() {
	fmt.Printf("\n=== 向量组装器配置 ===\n")
	fmt.Printf("向量大小(s): %d\n", va.S)
	fmt.Printf("每图像特征数(k): %d\n", va.K)
	fmt.Printf("图像特征总数: %d\n", va.ImageFeatures)
	fmt.Printf("每向量图像数(s/k): %d\n", va.ImagesPerVector)
	fmt.Printf("向量总数(ceil(ImageFeatures/k)): %d\n", va.NumVectors)

	// 检查配置的合理性
	if va.ImageFeatures%va.K != 0 {
		fmt.Printf("⚠️  注意: %d不能被k整除，最后一个向量将补0\n", va.ImageFeatures)
		lastVectorFeatures := va.ImageFeatures - (va.NumVectors-1)*va.K
		fmt.Printf("   最后一个向量实际特征数: %d/%d\n", lastVectorFeatures, va.K)
	}

	if va.S%va.K != 0 {
		fmt.Printf("⚠️  注意: s不能被k整除，向量末尾将补0\n")
		usedPositions := va.ImagesPerVector * va.K
		fmt.Printf("   实际使用的向量位置: %d/%d\n", usedPositions, va.S)
	}

	// 显示重排模式
	fmt.Printf("\n重排模式:\n")
	fmt.Printf("vector1: [img1_pixel0, img2_pixel0, ..., img%d_pixel0, img1_pixel1, ..., img%d_pixel1, ..., img1_pixel%d, ..., img%d_pixel%d]\n",
		va.ImagesPerVector, va.ImagesPerVector, va.K-1, va.ImagesPerVector, va.K-1)
	fmt.Printf("vector2: [img1_pixel%d, img2_pixel%d, ..., img%d_pixel%d, ...]\n",
		va.K, va.K, va.ImagesPerVector, va.K)
	fmt.Printf("...\n")
	fmt.Printf("vector%d: [img1_pixel%d, img2_pixel%d, ..., img%d_pixel%d, ...]\n",
		va.NumVectors, (va.NumVectors-1)*va.K, (va.NumVectors-1)*va.K, va.ImagesPerVector, (va.NumVectors-1)*va.K)

	// 添加批次处理说明
	fmt.Printf("\n批次处理说明:\n")
	fmt.Printf("• 每个批次处理 %d 张图像\n", va.ImagesPerVector)
	fmt.Printf("• 每个批次产生 %d 个向量\n", va.NumVectors)
	fmt.Printf("• 总向量数 = 批次数 × %d\n", va.NumVectors)
	fmt.Printf("• 例如：50张图像需要 %d 个批次，总共 %d 个向量\n",
		(50+va.ImagesPerVector-1)/va.ImagesPerVector,
		((50+va.ImagesPerVector-1)/va.ImagesPerVector)*va.NumVectors)

	fmt.Printf("========================\n\n")
}

// min 辅助函数：返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
