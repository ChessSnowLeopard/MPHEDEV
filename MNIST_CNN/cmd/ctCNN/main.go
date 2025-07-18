package main

import (
	"MNIST-CNN/pkg/homomorphic"
	"MNIST-CNN/pkg/participant"
	"MNIST-CNN/pkg/protocols"
	"MNIST-CNN/pkg/setup"
	"MNIST-CNN/pkg/training"
	"MNIST-CNN/pkg/vector_processor"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

// Config 配置结构体
type Config struct {
	VectorSize int // 向量大小 (s)
	BatchSize  int // 批处理大小 (batch)
	K          int // 每个图像的特征数 (k)
}

// NewConfig 创建新的配置
func NewConfig(vectorSize int, batchOrK int) *Config {
	return &Config{
		VectorSize: vectorSize,
		BatchSize:  batchOrK,
		K:          vectorSize / batchOrK,
	}
}

// ValidateConfig 验证配置合理性
func (c *Config) ValidateConfig(imageFeatures int) {
	fmt.Printf("\n=== 配置验证 ===\n")
	fmt.Printf("向量大小(s): %d\n", c.VectorSize)
	fmt.Printf("批处理大小(batch): %d\n", c.BatchSize)
	fmt.Printf("每图像特征数(k): %d\n", c.K)
	fmt.Printf("图像特征总数: %d\n", imageFeatures)

	if c.VectorSize&(c.VectorSize-1) != 0 {
		fmt.Printf("⚠️  警告: 向量大小不是2的幂，可能影响性能\n")
	}

	if c.K > imageFeatures {
		fmt.Printf("⚠️  警告: k(%d) > %d，将浪费向量空间\n", c.K, imageFeatures)
	} else if c.K < imageFeatures {
		fmt.Printf("✓ k(%d) < %d，需要多个向量表示一张图像\n", c.K, imageFeatures)
		numVectors := (imageFeatures + c.K - 1) / c.K
		fmt.Printf("   每张图像需要向量数: %d\n", numVectors)
	}

	utilization := float64(imageFeatures) / float64(c.K) * 100
	fmt.Printf("向量利用率: %.1f%%\n", utilization)
	fmt.Printf("================\n")
}

func main() {
	fmt.Println("======================================================")
	fmt.Println("     MNIST多方同态加密系统 - 支持灵活旋转配置")
	fmt.Println("======================================================")
	fmt.Println()

	// 记录开始时间
	startTime := time.Now()

	// 设置随机种子（固定种子以便比较不同初始化方法）
	rand.Seed(42)

	// ============ 第一部分：系统初始化 ============
	fmt.Println("\n====== 第一部分：多方系统初始化 ======")

	// 1. 设置参与方数量
	N := 3
	fmt.Printf("\n1. 初始化多方系统（%d个参与方）...\n", N)

	// 配置旋转参数
	fmt.Println("\n配置旋转参数...")

	// 为隐藏层权重梯度计算添加必要的旋转距离
	// BatchSize=64时，需要的旋转距离：64, 128, 192, 256, 320, 384, 448, 512, 576, 640, ...
	additionalRotations := []int{}
	batchSize := 64
	numSegments := 8192 / batchSize // 总段数
	for i := 1; i < numSegments; i++ {
		rotation := i * batchSize
		if rotation <= 8192 {
			additionalRotations = append(additionalRotations, rotation)
		}
	}

	rotConfig := setup.RotationConfig{
		BasicRotations:    []int{1, 2, 3, 4, 5, 10, 20, 50, 100},
		BatchRotations:    append([]int{64, 128, 256, 512}, additionalRotations...),
		IncludePowerOfTwo: false,
		MaxRotation:       8192,
	}

	// 使用自定义旋转配置初始化系统
	params, parties, cloud, galEls, _, err := setup.InitializeSystemWithRotations(N, rotConfig)
	if err != nil {
		log.Fatalf("系统初始化失败: %v", err)
	}

	// 获取CKKS参数信息
	slots := 1 << params.LogMaxSlots()
	fmt.Printf("\n系统初始化完成\n")
	fmt.Printf("• CKKS参数: LogN=%d, Slots=%d\n", params.LogN(), slots)
	fmt.Printf("• 伽罗瓦元素数量: %d\n", len(galEls))

	// 生成聚合私钥（仅用于测试验证）
	skAgg := setup.GenerateAggregatedSecretKey(params, parties)

	// ============ 第二部分：数据加载和重排 ============
	fmt.Println("\n====== 第二部分：数据准备 ======")

	// 2. 加载MNIST数据集
	fmt.Println("\n2. 加载MNIST数据集...")
	trainDataset, testDataset, err := training.LoadDataset()
	if err != nil {
		log.Fatalf("加载数据集失败: %v", err)
	}

	fmt.Printf("训练数据集: %d 个样本\n", len(trainDataset.Images))
	fmt.Printf("测试数据集: %d 个样本\n", len(testDataset.Images))

	// 3. 配置向量组装器
	fmt.Println("\n3. 配置向量组装器...")
	config := NewConfig(slots, 64)
	config.ValidateConfig(784)

	// 创建向量组装器
	assembler := vector_processor.NewVectorAssemblerFromDataset(config.VectorSize, config.K, trainDataset)
	assembler.PrintVectorInfo()

	// 4. 重排数据集
	fmt.Println("\n4. 重排数据集...")

	// 为了演示，只处理部分数据
	numBatchesToProcess := 1
	smallTrainDataset := &training.Dataset{
		Images: trainDataset.Images[:assembler.ImagesPerVector*numBatchesToProcess],
		Labels: trainDataset.Labels[:assembler.ImagesPerVector*numBatchesToProcess],
	}

	trainVectors, trainBatchLabels, err := assembler.AssembleDataset(smallTrainDataset)
	if err != nil {
		log.Fatalf("重排训练数据集失败: %v", err)
	}

	// 测试集也只处理部分
	smallTestDataset := &training.Dataset{
		Images: testDataset.Images[:assembler.ImagesPerVector*2],
		Labels: testDataset.Labels[:assembler.ImagesPerVector*2],
	}
	testVectors, testBatchLabels, err := assembler.AssembleDataset(smallTestDataset)
	if err != nil {
		log.Fatalf("重排测试数据集失败: %v", err)
	}

	fmt.Printf("\n重排完成统计:")
	fmt.Printf("\n• 训练集: %d 个向量, %d 个批次", len(trainVectors), len(trainBatchLabels))
	fmt.Printf("\n• 测试集: %d 个向量, %d 个批次\n", len(testVectors), len(testBatchLabels))

	// ============ 第三部分：分布式密钥生成 ============
	fmt.Println("\n====== 第三部分：分布式密钥生成 ======")
	pk, rlk, gks := generateDistributedKeys(params, N, parties, cloud, galEls)

	// ============ 第四部分：数据加密 ============
	fmt.Println("\n====== 第四部分：数据加密 ======")
	encoder := ckks.NewEncoder(params)
	encryptor := ckks.NewEncryptor(params, pk)
	decryptor := ckks.NewDecryptor(params, skAgg)

	// 创建评估器
	evk := rlwe.NewMemEvaluationKeySet(rlk, gks...)
	evaluator := ckks.NewEvaluator(params, evk)

	// 加密数据
	encryptedTrainVectors := encryptVectors(trainVectors, encoder, encryptor, params, slots, "训练")

	// ============ 第五部分：同态神经网络训练 ============
	fmt.Println("\n====== 第五部分：同态神经网络训练 ======")

	// 需要恢复crs变量用于自举
	crs, err := sampling.NewPRNG()
	if err != nil {
		log.Fatalf("初始化随机数生成器失败: %v", err)
	}

	// 开始完整的多层神经网络训练
	testMultiLayerNetwork(params, encoder, decryptor, evaluator,
		encryptedTrainVectors, trainVectors, trainBatchLabels,
		assembler, slots, trainDataset,
		N, parties, cloud, crs)

	// 计算总执行时间
	duration := time.Since(startTime)

	fmt.Println("\n======================================================")
	fmt.Println("     系统执行总结")
	fmt.Println("======================================================")
	fmt.Println("✓ 多方系统初始化（含自定义旋转配置）")
	fmt.Println("✓ 数据集加载和重排")
	fmt.Println("✓ 分布式密钥生成")
	fmt.Println("✓ 数据加密")
	fmt.Println("✓ 同态神经网络多轮训练")
	fmt.Printf("\n总执行时间: %v\n", duration)
	fmt.Println("======================================================")
}

// testBootstrapping 测试分布式自举协议
func testBootstrapping(params ckks.Parameters, N int, parties []*participant.Party,
	cloud *participant.Cloud, encryptedVectors map[uint64]*rlwe.Ciphertext,
	originalVectors [][]float32, encoder *ckks.Encoder, decryptor *rlwe.Decryptor,
	crs *sampling.KeyedPRNG, slots int) {

	fmt.Println("\n=== 分布式自举协议测试 ===")

	// 选择部分密文进行自举测试（避免测试时间过长）
	numToBootstrap := min(3, len(encryptedVectors))
	fmt.Printf("选择 %d 个密文进行自举测试\n", numToBootstrap)

	// 准备待自举的密文
	ciphertextsToBootstrap := make(map[uint64]*rlwe.Ciphertext)
	originalData := make(map[uint64][]float32)

	vectorIdx := 0
	for key, ct := range encryptedVectors {
		if vectorIdx >= numToBootstrap {
			break
		}
		ciphertextsToBootstrap[key] = ct
		originalData[key] = originalVectors[vectorIdx]
		vectorIdx++
	}

	// 打印自举前的密文状态
	fmt.Printf("\n自举前密文状态:\n")
	for key, ct := range ciphertextsToBootstrap {
		fmt.Printf("  密文 %d: Level=%d, Scale=2^%.2f, Noise Budget≈%.1f bits\n",
			key, ct.Level(), ct.Scale.Log2(),
			float64(params.LogQ()))
	}

	// 执行分布式自举
	fmt.Printf("\n开始执行分布式自举...\n")
	bootstrapStart := time.Now()

	protocols.RefreshCiphertexts(params, N, parties, cloud, ciphertextsToBootstrap, crs)

	// 收集自举结果
	refreshedCiphertexts := make(map[uint64]*rlwe.Ciphertext)
	for i := 0; i < numToBootstrap; i++ {
		result := <-cloud.RefreshDone
		refreshedCiphertexts[result.Key] = result.Ciphertext
	}

	bootstrapTime := time.Since(bootstrapStart)
	fmt.Printf("自举完成，用时: %v\n", bootstrapTime)

	// 打印自举后的密文状态
	fmt.Printf("\n自举后密文状态:\n")
	for key, ct := range refreshedCiphertexts {
		fmt.Printf("  密文 %d: Level=%d, Scale=2^%.2f, Noise Budget≈%.1f bits\n",
			key, ct.Level(), ct.Scale.Log2(),
			float64(params.LogQ()))
	}

	// 验证自举结果的正确性
	fmt.Printf("\n验证自举结果的正确性:\n")
	allCorrect := true
	totalErrorSum := 0.0
	totalSamples := 0

	for key := range ciphertextsToBootstrap {
		originalCt := ciphertextsToBootstrap[key]
		refreshedCt := refreshedCiphertexts[key]
		originalVec := originalData[key]

		// 解密原始密文
		ptOriginal := decryptor.DecryptNew(originalCt)
		decodedOriginal := make([]complex128, slots)
		if err := encoder.Decode(ptOriginal, decodedOriginal); err != nil {
			fmt.Printf("  ❌ 密文 %d: 原始密文解码失败: %v\n", key, err)
			allCorrect = false
			continue
		}

		// 解密自举后的密文
		ptRefreshed := decryptor.DecryptNew(refreshedCt)
		decodedRefreshed := make([]complex128, slots)
		if err := encoder.Decode(ptRefreshed, decodedRefreshed); err != nil {
			fmt.Printf("  ❌ 密文 %d: 自举密文解码失败: %v\n", key, err)
			allCorrect = false
			continue
		}

		// 计算误差
		maxError := 0.0
		sumSquaredError := 0.0
		validSamples := 0

		numSamplesToCheck := min(len(originalVec), slots)
		for i := 0; i < numSamplesToCheck; i++ {
			decOriginal := real(decodedOriginal[i])
			decRefreshed := real(decodedRefreshed[i])

			// 计算自举前后的差异
			bootError := math.Abs(decOriginal - decRefreshed)

			maxError = math.Max(maxError, bootError)
			sumSquaredError += bootError * bootError
			validSamples++
		}

		if validSamples > 0 {
			rmseError := math.Sqrt(sumSquaredError / float64(validSamples))
			totalErrorSum += sumSquaredError
			totalSamples += validSamples

			fmt.Printf("  密文 %d: MaxError=%.3e, RMSE=%.3e",
				key, maxError, rmseError)

			if maxError < 1e-6 {
				fmt.Printf(" ✓\n")
			} else if maxError < 1e-3 {
				fmt.Printf(" ⚠️ (可接受)\n")
			} else {
				fmt.Printf(" ❌ (误差过大)\n")
				allCorrect = false
			}
		}
	}

	// 总体统计
	if totalSamples > 0 {
		overallRMSE := math.Sqrt(totalErrorSum / float64(totalSamples))
		fmt.Printf("\n总体统计:\n")
		fmt.Printf("  平均RMSE误差: %.3e\n", overallRMSE)
		fmt.Printf("  处理密文数: %d\n", len(refreshedCiphertexts))
		fmt.Printf("  总耗时: %v\n", bootstrapTime)
		fmt.Printf("  平均每密文耗时: %v\n", bootstrapTime/time.Duration(len(refreshedCiphertexts)))
	}

	if allCorrect {
		fmt.Printf("\n✓ 分布式自举测试通过！所有密文都被正确刷新。\n")
	} else {
		fmt.Printf("\n⚠️ 分布式自举测试中部分密文存在较大误差。\n")
	}

	// 额外验证：检查自举是否真的刷新了噪声预算
	fmt.Printf("\n自举效果验证:\n")
	for key := range ciphertextsToBootstrap {
		originalLevel := ciphertextsToBootstrap[key].Level()
		refreshedLevel := refreshedCiphertexts[key].Level()

		fmt.Printf("  密文 %d: Level %d -> %d",
			key, originalLevel, refreshedLevel)

		if refreshedLevel == params.MaxLevel() {
			fmt.Printf(" ✓ (噪声预算已刷新)\n")
		} else {
			fmt.Printf(" ⚠️ (未完全刷新)\n")
		}
	}

	fmt.Println("\n=== 分布式自举测试完成 ===")
}

// generateDistributedKeys 生成分布式密钥
func generateDistributedKeys(params ckks.Parameters, N int, parties []*participant.Party,
	cloud *participant.Cloud, galEls []uint64) (*rlwe.PublicKey, *rlwe.RelinearizationKey, []*rlwe.GaloisKey) {

	// 5.1 分布式公钥生成
	fmt.Println("\n5.1 执行分布式公钥生成协议")
	protocols.GeneratePublicKey(params, N, parties, cloud)
	pk := <-cloud.PkgDone
	fmt.Println("✓ 公钥生成完成")

	// 5.2 分布式伽罗瓦密钥生成
	fmt.Println("\n5.2 执行分布式伽罗瓦密钥生成协议")
	fmt.Printf("正在生成 %d 个伽罗瓦密钥...\n", len(galEls))
	protocols.GenerateGaloisKeys(params, N, parties, cloud, galEls)

	gks := []*rlwe.GaloisKey{}
	for range galEls {
		gk := <-cloud.GalKeyDone
		gks = append(gks, gk)
	}
	fmt.Printf("✓ %d个伽罗瓦密钥生成完成\n", len(gks))

	// 5.3 分布式重线性化密钥生成
	fmt.Println("\n5.3 执行分布式重线性化密钥生成协议")
	protocols.GenerateRelinearizationKey(params, N, parties, cloud)
	rlk := <-cloud.RlkDone
	fmt.Println("✓ 重线性化密钥生成完成")

	// 分发密钥到各参与方
	for _, party := range parties {
		party.Pk = pk
		party.Rlk = rlk
		party.Gks = gks
	}

	return pk, rlk, gks
}

// encryptVectors 加密向量数据
func encryptVectors(vectors [][]float32, encoder *ckks.Encoder, encryptor *rlwe.Encryptor,
	params ckks.Parameters, slots int, dataType string) map[uint64]*rlwe.Ciphertext {

	fmt.Printf("\n加密%s数据...\n", dataType)
	encryptionStart := time.Now()

	encryptedVectors := make(map[uint64]*rlwe.Ciphertext)

	for idx, vector := range vectors {
		if idx%10 == 0 {
			fmt.Printf("\r加密进度: %d/%d 个向量", idx+1, len(vectors))
		}

		// 确保向量长度与槽位数匹配
		complexVector := make([]complex128, slots)
		for i := 0; i < len(vector) && i < slots; i++ {
			complexVector[i] = complex(float64(vector[i]), 0)
		}

		// 编码和加密
		pt := ckks.NewPlaintext(params, params.MaxLevel())
		if err := encoder.Encode(complexVector, pt); err != nil {
			log.Fatalf("编码失败: %v", err)
		}

		ct, err := encryptor.EncryptNew(pt)
		if err != nil {
			log.Fatalf("加密失败: %v", err)
		}

		encryptedVectors[uint64(idx)] = ct
	}

	fmt.Printf("\n✓ %s数据加密完成，用时: %v\n", dataType, time.Since(encryptionStart))
	return encryptedVectors
}

// testHomomorphicFC 测试同态全连接层计算
func testHomomorphicFC(params ckks.Parameters, encoder *ckks.Encoder, decryptor *rlwe.Decryptor,
	evaluator *ckks.Evaluator, encryptedVectors map[uint64]*rlwe.Ciphertext,
	plainVectors [][]float32, batchLabels [][]byte, assembler *vector_processor.VectorAssembler,
	slots int, trainDataset *training.Dataset) {

	fmt.Println("\n10. 测试同态全连接层计算...")
	fmt.Println("\n=== 同态全连接层计算测试 ===")

	// 创建全连接层
	fcLayer := homomorphic.NewFCLayer(assembler.ImageFeatures, 10, assembler)

	fmt.Printf("\n全连接层参数：\n")
	fmt.Printf("• 输入特征数: %d\n", fcLayer.InputSize)
	fmt.Printf("• 隐藏层大小: %d\n", fcLayer.HiddenSize)
	fmt.Printf("• 槽数(s): %d\n", assembler.S)
	fmt.Printf("• 每密文特征数(k): %d\n", assembler.K)
	fmt.Printf("• 每密文样本数(n): %d\n", assembler.ImagesPerVector)
	fmt.Printf("• 每批次需要向量数: %d\n", assembler.NumVectors)

	// 选择第一个批次进行测试
	batchIdx := 0
	fmt.Printf("\n处理第 %d 个批次（包含 %d 个样本）...\n", batchIdx+1, assembler.ImagesPerVector)

	// 执行同态计算
	startTime := time.Now()
	neuronOutputs, err := fcLayer.ComputeEncrypted(encryptedVectors, batchIdx, params, evaluator, encoder, slots)
	if err != nil {
		log.Fatalf("同态计算失败: %v", err)
	}
	computeTime := time.Since(startTime)

	// 验证结果
	fcLayer.VerifyResults(neuronOutputs, decryptor, encoder, plainVectors, batchIdx, trainDataset, slots)

	// 打印统计信息
	fcLayer.PrintStatistics(computeTime, batchLabels, batchIdx)

	fmt.Println("\n=== 同态全连接层测试完成 ===")
}

// testMultiLayerNetwork 测试多层神经网络
func testMultiLayerNetwork(params ckks.Parameters, encoder *ckks.Encoder, decryptor *rlwe.Decryptor,
	evaluator *ckks.Evaluator, encryptedVectors map[uint64]*rlwe.Ciphertext,
	plainVectors [][]float32, batchLabels [][]byte, assembler *vector_processor.VectorAssembler,
	slots int, trainDataset *training.Dataset,
	// 新增参数：用于自举
	N int, parties []*participant.Party, cloud *participant.Cloud, crs *sampling.KeyedPRNG) {

	fmt.Println("\n====== 同态加密神经网络训练 ======")

	// 创建一个3层网络：784 -> 128 -> 10
	layerSizes := []int{784, 64, 10}
	// 激活函数配置：隐藏层使用sigmoid，输出层使用identity（后续会应用softmax）
	activations := []string{"sigmoid_chebyshev", "identity"}

	mlp := homomorphic.NewMLPWithActivations(layerSizes, activations, assembler)

	mlp.PrintArchitecture()

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🔄 开始多轮训练迭代")
	fmt.Println(strings.Repeat("=", 60))

	// 直接执行多轮训练
	performMultiEpochTraining(mlp, encryptedVectors, plainVectors, batchLabels, assembler,
		params, evaluator, encoder, decryptor, slots, N, parties, cloud, crs)

	fmt.Println("\n=== 同态加密神经网络训练完成 ===")
}

// performMultiEpochTraining 执行多轮训练迭代
func performMultiEpochTraining(mlp *homomorphic.MLP, encryptedVectors map[uint64]*rlwe.Ciphertext,
	plainVectors [][]float32, batchLabels [][]byte, assembler *vector_processor.VectorAssembler,
	params ckks.Parameters, evaluator *ckks.Evaluator, encoder *ckks.Encoder, decryptor *rlwe.Decryptor,
	slots int, N int, parties []*participant.Party, cloud *participant.Cloud, crs *sampling.KeyedPRNG) {

	// 训练配置
	numEpochs := 15    // 训练轮数
	initialLR := 0.1   // 初始学习率
	lrDecayRate := 0.9 // 学习率衰减率
	lrDecayEpochs := 2 // 每隔几轮衰减学习率

	fmt.Printf("\n📊 多轮训练配置:\n")
	fmt.Printf("• 训练轮数: %d\n", numEpochs)
	fmt.Printf("• 初始学习率: %.4f\n", initialLR)
	fmt.Printf("• 学习率衰减: 每%d轮衰减%.1f%%\n", lrDecayEpochs, (1-lrDecayRate)*100)
	fmt.Printf("• 批次大小: %d\n", assembler.ImagesPerVector)
	fmt.Printf("• 网络结构: %d -> %d -> %d\n",
		mlp.Layers[0].InputSize, mlp.Layers[0].HiddenSize, mlp.Layers[1].HiddenSize)

	// 创建学习率调度器
	scheduler := homomorphic.NewStepLRScheduler(initialLR, lrDecayEpochs, lrDecayRate)

	// 训练历史记录
	trainingHistory := make([]TrainingEpochResult, 0, numEpochs)

	// 总训练开始时间
	totalTrainingStart := time.Now()

	// 开始多轮训练
	for epoch := 0; epoch < numEpochs; epoch++ {
		epochStart := time.Now()

		fmt.Printf("\n" + strings.Repeat("=", 50))
		fmt.Printf("\n🚀 Epoch %d/%d", epoch+1, numEpochs)
		fmt.Printf("\n" + strings.Repeat("=", 50))

		// 获取当前学习率
		currentLR := scheduler.GetLR()
		mlp.SetLearningRate(currentLR)
		fmt.Printf("\n📈 当前学习率: %.6f\n", currentLR)

		// 执行一轮训练
		epochResult, err := trainOneEpochComplete(mlp, encryptedVectors, plainVectors, batchLabels,
			assembler, params, evaluator, encoder, decryptor, slots, N, parties, cloud, crs, epoch+1)

		if err != nil {
			fmt.Printf("❌ Epoch %d 训练失败: %v\n", epoch+1, err)
			break
		}

		// 记录本轮结果
		epochResult.Epoch = epoch + 1
		epochResult.LearningRate = currentLR
		epochResult.EpochTime = time.Since(epochStart)
		trainingHistory = append(trainingHistory, epochResult)

		// 打印本轮结果
		fmt.Printf("\n📊 Epoch %d 结果:\n", epoch+1)
		fmt.Printf("• 损失值: %.6f\n", epochResult.Loss)
		fmt.Printf("• 准确率: %.2f%%\n", epochResult.Accuracy*100)
		fmt.Printf("• 学习率: %.6f\n", epochResult.LearningRate)
		fmt.Printf("• 用时: %v\n", epochResult.EpochTime)
		fmt.Printf("• 平均每样本用时: %.2fms\n",
			float64(epochResult.EpochTime.Nanoseconds())/float64(assembler.ImagesPerVector)/1e6)

		// 更新学习率调度器
		scheduler.Step()

		// 如果不是最后一轮，添加分隔符
		if epoch < numEpochs-1 {
			fmt.Printf("\n⏳ 准备下一轮训练...\n")
		}
	}

	totalTrainingTime := time.Since(totalTrainingStart)

	// 打印训练总结
	printTrainingSummary(trainingHistory, totalTrainingTime, assembler.ImagesPerVector)
}

// TrainingEpochResult 单轮训练结果
type TrainingEpochResult struct {
	Epoch        int           // 轮次
	Loss         float64       // 损失值
	Accuracy     float64       // 准确率
	LearningRate float64       // 学习率
	EpochTime    time.Duration // 本轮用时
}

// trainOneEpochComplete 执行一轮完整训练
func trainOneEpochComplete(mlp *homomorphic.MLP, encryptedVectors map[uint64]*rlwe.Ciphertext,
	plainVectors [][]float32, batchLabels [][]byte, assembler *vector_processor.VectorAssembler,
	params ckks.Parameters, evaluator *ckks.Evaluator, encoder *ckks.Encoder, decryptor *rlwe.Decryptor,
	slots int, N int, parties []*participant.Party, cloud *participant.Cloud, crs *sampling.KeyedPRNG,
	epoch int) (TrainingEpochResult, error) {

	var result TrainingEpochResult

	// 获取可用的批次数
	numBatches := len(batchLabels)
	if numBatches == 0 {
		return result, fmt.Errorf("没有可用的训练批次")
	}

	fmt.Printf("\n🔄 开始第%d轮训练，共%d个批次\n", epoch, numBatches)

	// 创建批次索引的随机排列（实现随机采样）
	batchIndices := make([]int, numBatches)
	for i := 0; i < numBatches; i++ {
		batchIndices[i] = i
	}
	// 使用固定的随机种子，确保可重现性，但通过epoch参数实现不同的随机排列
	r := rand.New(rand.NewSource(42))
	for i := len(batchIndices) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		j = (j + epoch) % (i + 1) // 通过epoch调整，确保每轮都有不同的批次顺序
		batchIndices[i], batchIndices[j] = batchIndices[j], batchIndices[i]
	}

	// 累积损失和准确率
	totalLoss := 0.0
	totalAccuracy := 0.0
	processedBatches := 0

	// 遍历所有批次
	for batchIdx := 0; batchIdx < numBatches; batchIdx++ {
		actualBatchIdx := batchIndices[batchIdx]
		labels := batchLabels[actualBatchIdx][:assembler.ImagesPerVector]

		fmt.Printf("\n🔄 处理批次 %d/%d（实际批次%d，%d个样本）\n",
			batchIdx+1, numBatches, actualBatchIdx+1, assembler.ImagesPerVector)

		// 步骤1：前向传播
		fmt.Printf("📊 步骤1: 前向传播...\n")
		forwardStart := time.Now()
		forwardResults, err := mlp.ForwardPropagationWithIntermediates(encryptedVectors, actualBatchIdx, params, evaluator, encoder, slots)
		if err != nil {
			fmt.Printf("⚠️ 批次%d前向传播失败: %v，跳过此批次\n", actualBatchIdx+1, err)
			continue
		}
		forwardTime := time.Since(forwardStart)

		// 步骤2：Softmax计算
		fmt.Printf("📊 步骤2: Softmax计算...\n")
		softmaxStart := time.Now()
		softmaxOutputs, err := homomorphic.HomomorphicSoftmaxWithBootstrap(
			forwardResults.FinalOutputs, params, evaluator, encoder, slots, N, parties, cloud, crs)
		if err != nil {
			fmt.Printf("⚠️ 批次%d Softmax计算失败: %v，跳过此批次\n", actualBatchIdx+1, err)
			continue
		}
		softmaxTime := time.Since(softmaxStart)

		// 步骤3：损失计算
		fmt.Printf("📊 步骤3: 交叉熵损失计算...\n")
		lossStart := time.Now()
		crossEntropyLoss := homomorphic.NewCrossEntropyLoss(10, assembler.ImagesPerVector, slots, assembler.K)
		lossVec, err := crossEntropyLoss.ComputeCrossEntropyLossPrivacyPreserving(
			softmaxOutputs, labels, params, evaluator, encoder, N, parties, cloud, crs)
		if err != nil {
			fmt.Printf("⚠️ 批次%d损失计算失败: %v，跳过此批次\n", actualBatchIdx+1, err)
			continue
		}

		// 解密损失值
		lossPt := decryptor.DecryptNew(lossVec)
		lossDecoded := make([]complex128, slots)
		if err := encoder.Decode(lossPt, lossDecoded); err != nil {
			fmt.Printf("⚠️ 批次%d损失解码失败: %v，跳过此批次\n", actualBatchIdx+1, err)
			continue
		}
		batchLoss := real(lossDecoded[0])
		totalLoss += batchLoss

		lossTime := time.Since(lossStart)

		// 步骤4：计算准确率
		batchAccuracy := calculateAccuracyFromSoftmax(softmaxOutputs, labels, decryptor, encoder, slots)
		totalAccuracy += batchAccuracy

		// 步骤5：梯度计算
		fmt.Printf("📊 步骤4: 梯度计算...\n")
		gradientStart := time.Now()

		// 输出层梯度
		gradientEngine := homomorphic.NewOutputGradientEngine(10, assembler.ImagesPerVector, slots)
		outputGradients, err := gradientEngine.ComputeOutputLayerGradients(
			softmaxOutputs, labels, params, evaluator, encoder)
		if err != nil {
			fmt.Printf("⚠️ 批次%d输出层梯度计算失败: %v，跳过此批次\n", actualBatchIdx+1, err)
			continue
		}

		// 重线性化输出梯度
		relinearizedOutputGradients := make([]*rlwe.Ciphertext, len(outputGradients))
		for i, grad := range outputGradients {
			relinearizedGrad := grad.CopyNew()
			if err := evaluator.Relinearize(relinearizedGrad, relinearizedGrad); err != nil {
				fmt.Printf("⚠️ 批次%d重线性化失败: %v，跳过此批次\n", actualBatchIdx+1, err)
				continue
			}
			relinearizedOutputGradients[i] = relinearizedGrad
		}

		gradientTime := time.Since(gradientStart)

		// 步骤6：权重梯度计算
		fmt.Printf("📊 步骤5: 权重梯度计算...\n")
		weightGradientStart := time.Now()

		// 输出层权重梯度
		outputWeightGradients, err := mlp.ComputeOutputWeightGradients(
			relinearizedOutputGradients, forwardResults.HiddenActivations, params, evaluator, encoder)
		if err != nil {
			fmt.Printf("⚠️ 批次%d输出层权重梯度计算失败: %v，跳过此批次\n", actualBatchIdx+1, err)
			continue
		}

		// 隐藏层梯度计算（只有在需要时）
		var hiddenWeightGradients [][]*rlwe.Ciphertext
		hiddenLayerSize := mlp.Layers[0].HiddenSize
		outputLayerSize := mlp.Layers[1].HiddenSize
		outputWeights := mlp.Layers[1].Weights
		hiddenActivation := mlp.Layers[0].Activation

		hiddenGradEngine := homomorphic.NewHiddenGradientEngineNeuronFormat(
			hiddenLayerSize, outputLayerSize, assembler.ImagesPerVector, slots, assembler.K)

		hiddenGradients, err := hiddenGradEngine.ComputeHiddenLayerGradients(
			relinearizedOutputGradients, forwardResults.HiddenInputs, outputWeights,
			hiddenActivation, params, evaluator, encoder)
		if err != nil {
			fmt.Printf("⚠️ 批次%d隐藏层梯度计算失败: %v\n", actualBatchIdx+1, err)
		} else {
			// 计算隐藏层权重梯度
			hiddenWeightGradEngine := homomorphic.NewHiddenWeightGradientEngine(
				hiddenLayerSize, assembler.ImageFeatures, assembler.ImagesPerVector, slots, assembler.K)

			hiddenWeightGradients, err = hiddenWeightGradEngine.ComputeHiddenWeightGradients(
				hiddenGradients, encryptedVectors, actualBatchIdx, params, evaluator, encoder)
			if err != nil {
				fmt.Printf("⚠️ 批次%d隐藏层权重梯度计算失败: %v\n", actualBatchIdx+1, err)
				hiddenWeightGradients = nil
			}
		}

		weightGradientTime := time.Since(weightGradientStart)

		// 步骤7：参数更新
		fmt.Printf("📊 步骤6: 参数更新...\n")
		updateStart := time.Now()

		err = mlp.UpdateAllWeights(hiddenWeightGradients, outputWeightGradients, decryptor, encoder)
		if err != nil {
			fmt.Printf("⚠️ 批次%d参数更新失败: %v，跳过此批次\n", actualBatchIdx+1, err)
			continue
		}

		updateTime := time.Since(updateStart)

		fmt.Printf("✅ 批次%d完成: 损失=%.6f, 准确率=%.2f%%, 用时: 前向=%v, Softmax=%v, 损失=%v, 梯度=%v, 权重梯度=%v, 更新=%v\n",
			actualBatchIdx+1, batchLoss, batchAccuracy*100, forwardTime, softmaxTime, lossTime, gradientTime, weightGradientTime, updateTime)

		processedBatches++
	}

	// 计算平均损失和准确率
	if processedBatches > 0 {
		result.Loss = totalLoss / float64(processedBatches)
		result.Accuracy = totalAccuracy / float64(processedBatches)
		fmt.Printf("✅ 第%d轮训练完成: 处理%d/%d批次, 平均损失=%.6f, 平均准确率=%.2f%%\n",
			epoch, processedBatches, numBatches, result.Loss, result.Accuracy*100)
	} else {
		return result, fmt.Errorf("第%d轮训练失败：没有成功处理任何批次", epoch)
	}

	return result, nil
}

// calculateAccuracyFromSoftmax 从softmax输出计算准确率
func calculateAccuracyFromSoftmax(softmaxOutputs []*rlwe.Ciphertext, labels []byte,
	decryptor *rlwe.Decryptor, encoder *ckks.Encoder, slots int) float64 {

	if len(softmaxOutputs) == 0 || len(labels) == 0 {
		return 0.0
	}

	batchSize := len(labels)
	correct := 0

	// 解密预测结果
	predictions := make([][]float64, len(softmaxOutputs))
	for i, ct := range softmaxOutputs {
		pt := decryptor.DecryptNew(ct)
		decoded := make([]complex128, slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			continue
		}

		predictions[i] = make([]float64, batchSize)
		for j := 0; j < batchSize; j++ {
			predictions[i][j] = real(decoded[j])
		}
	}

	// 计算每个样本的预测类别
	for sampleIdx := 0; sampleIdx < batchSize; sampleIdx++ {
		maxProb := -1.0
		predictedClass := -1

		for classIdx := 0; classIdx < len(predictions); classIdx++ {
			if predictions[classIdx][sampleIdx] > maxProb {
				maxProb = predictions[classIdx][sampleIdx]
				predictedClass = classIdx
			}
		}

		if predictedClass == int(labels[sampleIdx]) {
			correct++
		}
	}

	return float64(correct) / float64(batchSize)
}

// printTrainingSummary 打印训练总结
func printTrainingSummary(history []TrainingEpochResult, totalTime time.Duration, batchSize int) {
	fmt.Printf("\n" + strings.Repeat("=", 60))
	fmt.Printf("\n🎉 多轮训练完成总结")
	fmt.Printf("\n" + strings.Repeat("=", 60))

	fmt.Printf("\n📊 训练历史:\n")
	fmt.Printf("Epoch | 损失值   | 准确率  | 学习率   | 用时    \n")
	fmt.Printf("------|---------|---------|---------|----------\n")

	for _, epoch := range history {
		fmt.Printf("%5d | %7.4f | %6.2f%% | %.6f | %8s\n",
			epoch.Epoch, epoch.Loss, epoch.Accuracy*100, epoch.LearningRate,
			epoch.EpochTime.Truncate(time.Millisecond))
	}

	if len(history) > 0 {
		// 计算改进情况
		firstEpoch := history[0]
		lastEpoch := history[len(history)-1]

		lossImprovement := firstEpoch.Loss - lastEpoch.Loss
		accuracyImprovement := (lastEpoch.Accuracy - firstEpoch.Accuracy) * 100

		fmt.Printf("\n📈 训练改进:\n")
		fmt.Printf("• 损失变化: %.4f -> %.4f (改进: %.4f)\n",
			firstEpoch.Loss, lastEpoch.Loss, lossImprovement)
		fmt.Printf("• 准确率变化: %.2f%% -> %.2f%% (改进: %.2f%%)\n",
			firstEpoch.Accuracy*100, lastEpoch.Accuracy*100, accuracyImprovement)

		// 平均统计
		avgLoss := 0.0
		avgAccuracy := 0.0
		avgTime := time.Duration(0)

		for _, epoch := range history {
			avgLoss += epoch.Loss
			avgAccuracy += epoch.Accuracy
			avgTime += epoch.EpochTime
		}

		avgLoss /= float64(len(history))
		avgAccuracy /= float64(len(history))
		avgTime /= time.Duration(len(history))

		fmt.Printf("\n📊 平均统计:\n")
		fmt.Printf("• 平均损失: %.4f\n", avgLoss)
		fmt.Printf("• 平均准确率: %.2f%%\n", avgAccuracy*100)
		fmt.Printf("• 平均每轮用时: %v\n", avgTime.Truncate(time.Millisecond))
		fmt.Printf("• 平均每样本用时: %.2fms\n",
			float64(avgTime.Nanoseconds())/float64(batchSize)/1e6)
	}

	fmt.Printf("\n⏱️ 总体时间统计:\n")
	fmt.Printf("• 总训练时间: %v\n", totalTime.Truncate(time.Millisecond))
	if len(history) > 0 {
		fmt.Printf("• 总训练轮数: %d\n", len(history))
		fmt.Printf("• 总训练样本: %d\n", len(history)*batchSize)
		fmt.Printf("• 平均每轮时间: %v\n",
			(totalTime / time.Duration(len(history))).Truncate(time.Millisecond))
		fmt.Printf("• 平均每样本时间: %.2fms\n",
			float64(totalTime.Nanoseconds())/float64(len(history)*batchSize)/1e6)
	}

	fmt.Printf("\n🏆 训练完成状态:\n")
	if len(history) > 0 {
		lastEpoch := history[len(history)-1]
		fmt.Printf("• 最终损失: %.6f\n", lastEpoch.Loss)
		fmt.Printf("• 最终准确率: %.2f%%\n", lastEpoch.Accuracy*100)
		fmt.Printf("• 最终学习率: %.6f\n", lastEpoch.LearningRate)

		if lastEpoch.Accuracy > 0.5 {
			fmt.Printf("• 训练状态: 🟢 良好（准确率 > 50%%）\n")
		} else if lastEpoch.Accuracy > 0.3 {
			fmt.Printf("• 训练状态: 🟡 一般（准确率 30-50%%）\n")
		} else {
			fmt.Printf("• 训练状态: 🔴 需要改进（准确率 < 30%%）\n")
		}
	}

	fmt.Printf("\n✅ 多轮训练成功完成！\n")
}

// ForwardPropagationWithBootstrap 带自举的前向传播
func ForwardPropagationWithBootstrap(
	mlp *homomorphic.MLP,
	encryptedVectors map[uint64]*rlwe.Ciphertext,
	batchIdx int,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	slots int,
	N int,
	parties []*participant.Party,
	cloud *participant.Cloud,
	crs *sampling.KeyedPRNG) ([]*rlwe.Ciphertext, error) {

	fmt.Println("\n=== 开始带自举的多层前向传播 ===")
	startTime := time.Now()

	var currentOutputs []*rlwe.Ciphertext

	// 处理每一层
	for layerIdx := 0; layerIdx < len(mlp.Layers); layerIdx++ {
		layer := mlp.Layers[layerIdx]
		fmt.Printf("\n--- 处理第%d层（%d -> %d）---\n", layerIdx+1, layer.InputSize, layer.HiddenSize)
		fmt.Printf("激活函数: %s\n", layer.Activation.GetName())

		// 计算当前层
		var layerOutputs []*rlwe.Ciphertext
		var err error

		if layerIdx == 0 {
			// 第一层：带重组
			layerOutputs, err = layer.ComputeEncryptedWithReorganization(
				encryptedVectors, batchIdx, params, evaluator, encoder, slots)
		} else if layerIdx < len(mlp.Layers)-1 {
			// 中间层：计算并重组
			neuronOutputs, err := layer.ComputeFromReorganized(
				currentOutputs, params, evaluator, encoder, slots)
			if err == nil {
				reorganizer := homomorphic.NewMaskReorganizer(mlp.Assembler.K, mlp.Assembler.ImagesPerVector, slots)
				layerOutputs, err = reorganizer.ReorganizeOutputs(neuronOutputs, params, encoder, evaluator)
			}
		} else {
			// 最后一层：不重组
			layerOutputs, err = layer.ComputeFromReorganized(
				currentOutputs, params, evaluator, encoder, slots)
		}

		if err != nil {
			return nil, fmt.Errorf("第%d层计算失败: %v", layerIdx+1, err)
		}

		// 检查是否需要自举
		minLevel := params.MaxLevel()
		for _, ct := range layerOutputs {
			if ct.Level() < minLevel {
				minLevel = ct.Level()
			}
		}

		// 如果层级太低（小于最大层级的一半），执行自举
		if minLevel < params.MaxLevel()/2 {
			fmt.Printf("\n密文层级较低（最低为%d/%d），执行自举刷新...\n", minLevel, params.MaxLevel())

			// 将输出转换为map格式
			outputMap := make(map[uint64]*rlwe.Ciphertext)
			for i, ct := range layerOutputs {
				outputMap[uint64(i)] = ct
			}

			// 执行自举
			protocols.RefreshCiphertexts(params, N, parties, cloud, outputMap, crs)

			// 收集自举后的结果
			refreshedOutputs := make([]*rlwe.Ciphertext, len(layerOutputs))
			for i := 0; i < len(layerOutputs); i++ {
				result := <-cloud.RefreshDone
				refreshedOutputs[result.Key] = result.Ciphertext
			}

			layerOutputs = refreshedOutputs
			fmt.Printf("✓ 自举完成，密文层级已刷新\n")
		} else {
			fmt.Printf("密文层级充足（最低为%d/%d），无需自举\n", minLevel, params.MaxLevel())
		}

		currentOutputs = layerOutputs
	}

	duration := time.Since(startTime)
	fmt.Printf("\n✓ 带自举的前向传播完成，总用时: %v\n", duration)

	return currentOutputs, nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// analyzeOutputGradientProperties 分析输出层梯度特性
func analyzeOutputGradientProperties(outputGradients []*rlwe.Ciphertext, labels []byte,
	decryptor *rlwe.Decryptor, encoder *ckks.Encoder, slots int, batchSize int) {

	// 解密梯度
	gradients := make([][]float64, len(outputGradients))
	for j := 0; j < len(outputGradients); j++ {
		pt := decryptor.DecryptNew(outputGradients[j])
		decoded := make([]complex128, slots)
		encoder.Decode(pt, decoded)

		gradients[j] = make([]float64, batchSize)
		for i := 0; i < batchSize; i++ {
			gradients[j][i] = real(decoded[i])
		}
	}

	// 分析梯度特性
	fmt.Println("梯度特性分析（前3个样本）：")
	for i := 0; i < min(3, batchSize); i++ {
		trueLabel := int(labels[i])
		fmt.Printf("\n样本%d (真实标签: %d):\n", i, trueLabel)

		gradientSum := 0.0
		maxGradient := gradients[0][i]
		minGradient := gradients[0][i]

		for j := 0; j < len(gradients); j++ {
			grad := gradients[j][i]
			gradientSum += grad

			if grad > maxGradient {
				maxGradient = grad
			}
			if grad < minGradient {
				minGradient = grad
			}

			marker := ""
			if j == trueLabel {
				marker = " ← 真实类别"
			}

			fmt.Printf("  类别%d: %8.6f%s\n", j, grad, marker)
		}

		fmt.Printf("  梯度和: %8.6f (理论值: 0)\n", gradientSum)
		fmt.Printf("  梯度范围: [%.6f, %.6f]\n", minGradient, maxGradient)
	}

	// 统计所有样本的梯度特性
	fmt.Println("\n整体梯度统计：")
	totalGradientSum := 0.0
	totalAbsGradientSum := 0.0
	maxAbsGradient := 0.0

	for i := 0; i < batchSize; i++ {
		sampleGradientSum := 0.0
		for j := 0; j < len(gradients); j++ {
			grad := gradients[j][i]
			sampleGradientSum += grad
			absGrad := math.Abs(grad)
			totalAbsGradientSum += absGrad
			if absGrad > maxAbsGradient {
				maxAbsGradient = absGrad
			}
		}
		totalGradientSum += math.Abs(sampleGradientSum)
	}

	avgAbsGradient := totalAbsGradientSum / float64(batchSize*len(gradients))
	avgGradientSumError := totalGradientSum / float64(batchSize)

	fmt.Printf("• 平均绝对梯度值: %.6f\n", avgAbsGradient)
	fmt.Printf("• 最大绝对梯度值: %.6f\n", maxAbsGradient)
	fmt.Printf("• 平均梯度和误差: %.8f (理论值: 0)\n", avgGradientSumError)

	// 梯度分布分析
	positiveCount := 0
	negativeCount := 0
	zeroCount := 0

	for i := 0; i < batchSize; i++ {
		for j := 0; j < len(gradients); j++ {
			grad := gradients[j][i]
			if grad > 1e-8 {
				positiveCount++
			} else if grad < -1e-8 {
				negativeCount++
			} else {
				zeroCount++
			}
		}
	}

	total := positiveCount + negativeCount + zeroCount
	fmt.Printf("• 梯度分布: 正值 %.1f%%, 负值 %.1f%%, 零值 %.1f%%\n",
		float64(positiveCount)/float64(total)*100,
		float64(negativeCount)/float64(total)*100,
		float64(zeroCount)/float64(total)*100)
}
