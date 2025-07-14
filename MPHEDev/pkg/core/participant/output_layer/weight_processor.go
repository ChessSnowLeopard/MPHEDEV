package output_layer

import (
	"MPHEDev/pkg/core/participant/activation"
	"MPHEDev/pkg/core/participant/interfaces"
	"fmt"
	"math"

	"MPHEDev/pkg/core/participant/crypto"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// OutputLayerWeightProcessor 输出层权重处理器
type OutputLayerWeightProcessor struct {
	weightManager      interfaces.WeightManager
	dataPackingService interfaces.DataPackingService

	// 输出层配置
	InputSize  int // 输入特征数（来自隐藏层）
	OutputSize int // 输出层神经元数（类别数）

	// 权重矩阵（明文）
	Weights [][]float64 // 权重矩阵
	Biases  []float64   // 偏置向量

	// 激活函数组件
	softmaxProcessor *activation.SoftmaxProcessor
	crossEntropyLoss *activation.CrossEntropyLoss

	// 密钥管理器引用
	keyManager *crypto.KeyManager

	// 刷新服务引用
	refreshService *crypto.RefreshService

	// 网络相关
	onlinePeers map[int]string
	myID        int
}

// NewOutputLayerWeightProcessor 创建输出层权重处理器
func NewOutputLayerWeightProcessor(
	weightManager interfaces.WeightManager,
	dataPackingService interfaces.DataPackingService,
	inputSize, outputSize int) *OutputLayerWeightProcessor {

	// 创建激活函数组件
	softmaxProcessor := activation.NewSoftmaxProcessor()
	crossEntropyLoss := activation.NewCrossEntropyLoss(
		outputSize,                                // 类别数
		dataPackingService.GetBatchSize(),         // 批大小
		dataPackingService.GetSlotCount(),         // 密文槽数
		dataPackingService.GetFeaturesPerCipher(), // 每密文特征数
	)

	return &OutputLayerWeightProcessor{
		weightManager:      weightManager,
		dataPackingService: dataPackingService,
		InputSize:          inputSize,
		OutputSize:         outputSize,
		Weights:            nil,
		Biases:             nil,
		softmaxProcessor:   softmaxProcessor,
		crossEntropyLoss:   crossEntropyLoss,
		refreshService:     nil,
		onlinePeers:        nil,
		myID:               -1,
	}
}

// InitializeWeights 初始化权重矩阵
func (olwp *OutputLayerWeightProcessor) InitializeWeights(initMethod int) error {
	fmt.Printf("输出层：初始化权重矩阵 (%d x %d)...\n", olwp.InputSize, olwp.OutputSize)

	// 创建权重矩阵
	olwp.Weights = olwp.weightManager.CreateWeightMatrixWithMethod(olwp.OutputSize, olwp.InputSize, initMethod)

	// 创建偏置向量
	olwp.Biases = olwp.weightManager.CreateBiasVector(olwp.OutputSize)

	fmt.Printf("✓ 输出层权重矩阵初始化完成\n")
	fmt.Printf("  • 权重矩阵: %d x %d\n", len(olwp.Weights), len(olwp.Weights[0]))
	fmt.Printf("  • 偏置向量: %d\n", len(olwp.Biases))

	return nil
}

// ComputeOutputLayer 执行输出层计算
func (olwp *OutputLayerWeightProcessor) ComputeOutputLayer(
	encryptedVectors map[uint64]*rlwe.Ciphertext,
	batchIdx int,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder) ([]*rlwe.Ciphertext, error) {

	if olwp.Weights == nil {
		return nil, fmt.Errorf("权重矩阵未初始化")
	}

	fmt.Printf("输出层：执行前向计算（使用明文权重）...\n")
	fmt.Printf("计算参数:\n")
	fmt.Printf("  • 输入向量数: %d\n", len(encryptedVectors))
	fmt.Printf("  • 批次索引: %d\n", batchIdx)
	fmt.Printf("  • 权重矩阵: %d x %d\n", len(olwp.Weights), len(olwp.Weights[0]))
	fmt.Printf("  • 偏置向量: %d\n", len(olwp.Biases))

	// 获取配置参数
	n := olwp.dataPackingService.GetBatchSize() // 每向量图像数

	// 该批次对应的密文索引范围
	startVecIdx := batchIdx * olwp.dataPackingService.GetNumVectors()

	// 存储每个输出神经元的输出
	neuronOutputs := make([]*rlwe.Ciphertext, olwp.OutputSize)

	// 对每个输出神经元进行计算
	fmt.Printf("开始计算 %d 个输出神经元...\n", olwp.OutputSize)
	for neuronIdx := 0; neuronIdx < olwp.OutputSize; neuronIdx++ {
		fmt.Printf("  计算输出神经元 %d/%d...\n", neuronIdx+1, olwp.OutputSize)

		var accumulated *rlwe.Ciphertext

		// 处理每轮（每个向量）
		for round := 0; round < olwp.dataPackingService.GetNumVectors(); round++ {
			vecIdx := uint64(startVecIdx + round)

			// 获取当前轮的密文
			ct, exists := encryptedVectors[vecIdx]
			if !exists {
				fmt.Printf("    警告：密文 %d 不存在\n", vecIdx)
				continue
			}

			// 计算本轮处理的特征范围（参考MNIST_CNN项目）
			featureStart := round * olwp.dataPackingService.GetSlotCount() / n // 每向量特征数
			featureEnd := min(featureStart+olwp.dataPackingService.GetSlotCount()/n, olwp.InputSize)
			actualK := featureEnd - featureStart

			// 打包权重（使用明文权重，传递actualK参数）
			packedWeights := olwp.weightManager.PackWeightsForNeuron(olwp.Weights[neuronIdx], featureStart, actualK, n, olwp.dataPackingService.GetSlotCount())

			// 编码权重（明文编码，不加密）
			weightPt := ckks.NewPlaintext(params, params.MaxLevel())
			if err := encoder.Encode(packedWeights, weightPt); err != nil {
				fmt.Printf("    编码权重失败: %v\n", err)
				continue
			}

			// 密文与权重相乘（密文 × 明文 = 密文）
			mulResult, err := evaluator.MulNew(ct, weightPt)
			if err != nil {
				fmt.Printf("    密文乘法失败: %v\n", err)
				continue
			}

			// 乘法后立即rescale
			if err := evaluator.Rescale(mulResult, mulResult); err != nil {
				fmt.Printf("    Rescale失败: %v\n", err)
				continue
			}

			// 累加到结果
			if accumulated == nil {
				accumulated = mulResult
			} else {
				accumulated, err = evaluator.AddNew(accumulated, mulResult)
				if err != nil {
					fmt.Printf("    密文加法失败: %v\n", err)
					continue
				}
			}
		}

		// 执行旋转累加，将k个特征的结果相加
		result := accumulated
		for step := 0; step < int(math.Log2(float64(olwp.dataPackingService.GetSlotCount()/n))); step++ {
			rotateBy := n * (1 << step)
			rotated, err := evaluator.RotateNew(result, rotateBy)
			if err != nil {
				fmt.Printf("    旋转失败 (step=%d, rotate=%d): %v\n", step, rotateBy, err)
				continue
			}
			result, err = evaluator.AddNew(result, rotated)
			if err != nil {
				fmt.Printf("    累加失败: %v\n", err)
				continue
			}
		}

		// 添加偏置项
		if olwp.Biases != nil && neuronIdx < len(olwp.Biases) {
			// 打包偏置项
			packedBias := olwp.weightManager.PackBiasForNeuron(olwp.Biases[neuronIdx])

			// 编码偏置项（明文编码，不加密）
			biasPt := ckks.NewPlaintext(params, params.MaxLevel())
			if err := encoder.Encode(packedBias, biasPt); err != nil {
				fmt.Printf("    编码偏置项失败: %v\n", err)
			} else {
				// 添加偏置项到结果
				result, err = evaluator.AddNew(result, biasPt)
				if err != nil {
					fmt.Printf("    添加偏置项失败: %v\n", err)
				}
			}
		}

		neuronOutputs[neuronIdx] = result
		fmt.Printf("    ✓ 输出神经元 %d 计算完成\n", neuronIdx+1)
	}

	fmt.Printf("✓ 输出层前向计算完成，输出 %d 个神经元\n", len(neuronOutputs))
	return neuronOutputs, nil
}

// ComputeFromHiddenLayerOutput 从隐藏层输出格式计算输出层
// hiddenOutputs: 隐藏层输出的128个密文，每个密文格式为 [样本0_神经元i, 样本1_神经元i, ..., 样本127_神经元i | 重复k次]
func (olwp *OutputLayerWeightProcessor) ComputeFromHiddenLayerOutput(
	hiddenOutputs []*rlwe.Ciphertext,
	batchIdx int,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder) ([]*rlwe.Ciphertext, error) {

	if olwp.Weights == nil {

		return nil, fmt.Errorf("权重矩阵未初始化")
	}

	fmt.Printf("输出层：从隐藏层输出格式计算（使用明文权重）...\n")
	fmt.Printf("计算参数:\n")
	fmt.Printf("  • 隐藏层输出密文数: %d\n", len(hiddenOutputs))
	fmt.Printf("  • 批次索引: %d\n", batchIdx)
	fmt.Printf("  • 权重矩阵: %d x %d\n", len(olwp.Weights), len(olwp.Weights[0]))
	fmt.Printf("  • 偏置向量: %d\n", len(olwp.Biases))
	fmt.Printf("  • 输出层神经元数: %d\n", olwp.OutputSize)
	fmt.Printf("  • 输入特征数: %d\n", olwp.InputSize)

	// 验证隐藏层输出数量是否匹配
	if len(hiddenOutputs) != olwp.InputSize {
		fmt.Printf("⚠️  警告：隐藏层输出数量不匹配，期望 %d，实际 %d\n", olwp.InputSize, len(hiddenOutputs))
	}

	// 获取配置参数
	n := olwp.dataPackingService.GetBatchSize() // 每向量图像数
	fmt.Printf("  • 批次大小: %d\n", n)
	fmt.Printf("  • 密文槽数: %d\n", olwp.dataPackingService.GetSlotCount())

	// 存储每个输出神经元的输出
	neuronOutputs := make([]*rlwe.Ciphertext, olwp.OutputSize)

	// 对每个输出神经元进行计算
	fmt.Printf("开始计算 %d 个输出神经元...\n", olwp.OutputSize)

	for neuronIdx := 0; neuronIdx < olwp.OutputSize; neuronIdx++ {

		fmt.Printf("  计算输出神经元 %d/%d...\n", neuronIdx+1, olwp.OutputSize)

		// 验证权重矩阵访问
		if neuronIdx >= len(olwp.Weights) {
			fmt.Printf("    ❌ 错误：输出神经元 %d 超出权重矩阵范围 %d\n", neuronIdx, len(olwp.Weights))
			return nil, fmt.Errorf("输出神经元 %d 超出权重矩阵范围", neuronIdx)
		}

		var accumulated *rlwe.Ciphertext

		// 处理每个隐藏层神经元的输出密文
		fmt.Printf("    处理 %d 个隐藏层神经元输出...\n", len(hiddenOutputs))

		for hiddenNeuronIdx, hiddenOutput := range hiddenOutputs {
			if hiddenNeuronIdx%10 == 0 { // 每10个神经元打印一次进度
				fmt.Printf("      处理隐藏层神经元 %d/%d...\n", hiddenNeuronIdx+1, len(hiddenOutputs))
			}

			// 获取当前隐藏层神经元对应的权重
			if neuronIdx >= len(olwp.Weights) {
				fmt.Printf("    ❌ 错误：输出神经元 %d 超出权重矩阵范围 %d\n", neuronIdx, len(olwp.Weights))
				return nil, fmt.Errorf("输出神经元 %d 超出权重矩阵范围", neuronIdx)
			}

			if hiddenNeuronIdx >= len(olwp.Weights[neuronIdx]) {
				fmt.Printf("    ❌ 错误：隐藏层神经元 %d 超出权重范围 %d\n", hiddenNeuronIdx, len(olwp.Weights[neuronIdx]))
				return nil, fmt.Errorf("隐藏层神经元 %d 超出权重范围", hiddenNeuronIdx)
			}

			weight := olwp.Weights[neuronIdx][hiddenNeuronIdx]

			// 打包权重：将单个权重复制到所有样本位置
			packedWeights := make([]complex128, olwp.dataPackingService.GetSlotCount())
			for i := 0; i < n && i < len(packedWeights); i++ {
				packedWeights[i] = complex(weight, 0)
			}

			// 编码权重（明文编码，不加密）
			weightPt := ckks.NewPlaintext(params, params.MaxLevel())
			if err := encoder.Encode(packedWeights, weightPt); err != nil {
				fmt.Printf("    编码权重失败: %v\n", err)
				continue
			}

			// 密文与权重相乘（密文 × 明文 = 密文）
			mulResult, err := evaluator.MulNew(hiddenOutput, weightPt)
			if err != nil {
				fmt.Printf("    密文乘法失败: %v\n", err)
				continue
			}

			// 乘法后立即rescale
			if err := evaluator.Rescale(mulResult, mulResult); err != nil {
				fmt.Printf("    Rescale失败: %v\n", err)
				continue
			}

			// 累加到结果
			if accumulated == nil {
				accumulated = mulResult
			} else {
				accumulated, err = evaluator.AddNew(accumulated, mulResult)
				if err != nil {
					fmt.Printf("    密文加法失败: %v\n", err)
					continue
				}
			}

		}

		// 添加偏置项
		if olwp.Biases != nil && neuronIdx < len(olwp.Biases) {
			fmt.Printf("    添加偏置项...\n")
			// 打包偏置项
			packedBias := olwp.weightManager.PackBiasForNeuron(olwp.Biases[neuronIdx])

			// 编码偏置项（明文编码，不加密）
			biasPt := ckks.NewPlaintext(params, params.MaxLevel())
			if err := encoder.Encode(packedBias, biasPt); err != nil {
				fmt.Printf("    编码偏置项失败: %v\n", err)
			} else {
				// 添加偏置项到结果
				accumulated, err = evaluator.AddNew(accumulated, biasPt)
				if err != nil {
					fmt.Printf("    添加偏置项失败: %v\n", err)
				}
			}
		}

		neuronOutputs[neuronIdx] = accumulated
		fmt.Printf("    ✓ 输出神经元 %d 计算完成\n", neuronIdx+1)
	}

	fmt.Printf("✓ 输出层前向计算完成，输出 %d 个类别\n", len(neuronOutputs))

	return neuronOutputs, nil
}

// ApplySoftmax 应用softmax激活函数
func (olwp *OutputLayerWeightProcessor) ApplySoftmax(
	logits []*rlwe.Ciphertext,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder) ([]*rlwe.Ciphertext, error) {

	fmt.Printf("输出层：应用softmax激活函数...\n")

	// 检查输入密文的层级
	fmt.Printf("🔍 检查输入密文层级...\n")
	minInputLevel := params.MaxLevel()
	for i, ct := range logits {
		level := ct.Level()
		if level < minInputLevel {
			minInputLevel = level
		}
		fmt.Printf("  神经元%d: Level=%d/%d\n", i, level, params.MaxLevel())
	}
	fmt.Printf("  最低输入层级: %d/%d\n", minInputLevel, params.MaxLevel())

	// 设置自举阈值（当层级低于最大层级的1/3时触发刷新）
	bootstrapThreshold := params.MaxLevel() / 3

	// 检查是否需要自举
	if minInputLevel < bootstrapThreshold {
		fmt.Printf("🔄 检测到层级过低（%d < %d），触发自动刷新...\n", minInputLevel, bootstrapThreshold)

		if olwp.refreshService == nil {
			return nil, fmt.Errorf("刷新服务未设置，无法执行自动刷新")
		}

		if olwp.onlinePeers == nil || olwp.myID == -1 {
			return nil, fmt.Errorf("网络信息未设置，无法执行自动刷新")
		}

		// 执行同步刷新
		refreshedLogits, err := olwp.refreshService.RefreshCiphertextsSync(
			logits, olwp.onlinePeers, olwp.myID, "softmax_refresh")
		if err != nil {
			return nil, fmt.Errorf("自动刷新失败: %v", err)
		}

		fmt.Printf("✅ 自动刷新完成，使用刷新后的密文进行Softmax计算\n")
		logits = refreshedLogits
	} else {
		fmt.Printf("✅ 层级充足，无需刷新\n")
	}

	// 在调用SoftmaxProcessor之前，确保层级足够进行完整的Softmax计算
	// Softmax计算需要：exp计算 + 求和 + 倒数计算 + 乘法 + rescale
	// 通常需要至少5-6个层级
	requiredLevelForSoftmax := 6
	if minInputLevel < requiredLevelForSoftmax {
		fmt.Printf("🔄 检测到层级不足进行完整Softmax计算（%d < %d），再次刷新...\n", minInputLevel, requiredLevelForSoftmax)

		if olwp.refreshService == nil {
			return nil, fmt.Errorf("刷新服务未设置，无法执行自动刷新")
		}

		if olwp.onlinePeers == nil || olwp.myID == -1 {
			return nil, fmt.Errorf("网络信息未设置，无法执行自动刷新")
		}

		// 执行同步刷新
		refreshedLogits, err := olwp.refreshService.RefreshCiphertextsSync(
			logits, olwp.onlinePeers, olwp.myID, "softmax_full_refresh")
		if err != nil {
			return nil, fmt.Errorf("Softmax前自动刷新失败: %v", err)
		}

		fmt.Printf("✅ Softmax前自动刷新完成，使用刷新后的密文进行完整Softmax计算\n")
		logits = refreshedLogits
	} else {
		fmt.Printf("✅ 层级充足，可以进行完整Softmax计算\n")
	}

	// 使用已实现的SoftmaxProcessor
	softmaxOutputs, err := olwp.softmaxProcessor.ApplySoftmax(logits, params, evaluator, encoder)
	if err != nil {
		return nil, fmt.Errorf("softmax计算失败: %v", err)
	}

	fmt.Printf("✓ Softmax应用完成\n")
	return softmaxOutputs, nil
}

// ComputeLoss 计算交叉熵损失
func (olwp *OutputLayerWeightProcessor) ComputeLoss(
	softmaxOutputs []*rlwe.Ciphertext,
	labels []int,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder) (*rlwe.Ciphertext, error) {

	fmt.Printf("输出层：计算交叉熵损失...\n")

	// 使用已实现的CrossEntropyLoss
	loss, err := olwp.crossEntropyLoss.ComputeCrossEntropyLoss(softmaxOutputs, labels, params, evaluator, encoder)
	if err != nil {
		return nil, fmt.Errorf("交叉熵损失计算失败: %v", err)
	}

	fmt.Printf("✓ 交叉熵损失计算完成\n")
	return loss, nil
}

// ComputeLossWithEncryptedLabels 使用密文标签计算交叉熵损失
// encryptedLabels: 接收到的密文标签数据（one-hot编码后的密文）
func (olwp *OutputLayerWeightProcessor) ComputeLossWithEncryptedLabels(
	softmaxOutputs []*rlwe.Ciphertext, // P_0, P_1, ..., P_{t''-1}
	encryptedLabels []*rlwe.Ciphertext, // 密文标签（one-hot编码）
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder) (*rlwe.Ciphertext, error) {

	fmt.Printf("输出层：使用密文标签计算交叉熵损失...\n")
	fmt.Printf("输入参数:\n")
	fmt.Printf("  • Softmax输出数: %d\n", len(softmaxOutputs))
	fmt.Printf("  • 密文标签数: %d\n", len(encryptedLabels))

	// 验证输入
	if len(softmaxOutputs) != olwp.OutputSize {
		return nil, fmt.Errorf("softmax输出数量不匹配：期望 %d，实际 %d", olwp.OutputSize, len(softmaxOutputs))
	}

	if len(encryptedLabels) != olwp.OutputSize {
		return nil, fmt.Errorf("密文标签数量不匹配：期望 %d，实际 %d", olwp.OutputSize, len(encryptedLabels))
	}

	// 🔍 检查输入密文层级
	fmt.Printf("🔍 检查输入密文层级...\n")
	minInputLevel := params.MaxLevel()
	for i, ct := range softmaxOutputs {
		level := ct.Level()
		if level < minInputLevel {
			minInputLevel = level
		}
		fmt.Printf("  softmax输出%d: Level=%d/%d\n", i, level, params.MaxLevel())
	}
	fmt.Printf("  最低输入层级: %d/%d\n", minInputLevel, params.MaxLevel())

	// 设置自举阈值（当层级低于最大层级的1/3时触发刷新）
	bootstrapThreshold := params.MaxLevel() / 3

	// 检查是否需要自举
	if minInputLevel < bootstrapThreshold {
		fmt.Printf("🔄 检测到层级过低（%d < %d），触发自动刷新...\n", minInputLevel, bootstrapThreshold)

		if olwp.refreshService == nil {
			return nil, fmt.Errorf("刷新服务未设置，无法执行自动刷新")
		}

		if olwp.onlinePeers == nil || olwp.myID == -1 {
			return nil, fmt.Errorf("网络信息未设置，无法执行自动刷新")
		}

		// 执行同步刷新
		refreshedSoftmaxOutputs, err := olwp.refreshService.RefreshCiphertextsSync(
			softmaxOutputs, olwp.onlinePeers, olwp.myID, "crossentropy_refresh")
		if err != nil {
			return nil, fmt.Errorf("自动刷新失败: %v", err)
		}

		fmt.Printf("✅ 自动刷新完成，使用刷新后的密文进行交叉熵计算\n")
		softmaxOutputs = refreshedSoftmaxOutputs
	} else {
		fmt.Printf("✅ 层级充足，无需刷新\n")
	}

	// 步骤1: 对softmax输出计算对数
	fmt.Println("\n📊 步骤1：计算log(softmax)输出")
	logSoftmaxOutputs := make([]*rlwe.Ciphertext, len(softmaxOutputs))
	for i, softmaxOutput := range softmaxOutputs {
		fmt.Printf("  计算log(softmax_%d)...\n", i)
		logResult, err := olwp.crossEntropyLoss.LogApproximator.Apply(softmaxOutput, evaluator, params)
		if err != nil {
			return nil, fmt.Errorf("计算log(softmax_%d)失败: %v", i, err)
		}
		logSoftmaxOutputs[i] = logResult
		fmt.Printf("    log后层级: %d/%d\n", logResult.Level(), params.MaxLevel())
	}

	// 步骤2: 计算 -y_i * log(p_i) 的交叉熵项（密文标签 × 密文log）
	fmt.Println("\n📊 步骤2：计算交叉熵项 -y_i * log(p_i)")
	crossEntropyTerms := make([]*rlwe.Ciphertext, olwp.OutputSize)

	for i := 0; i < olwp.OutputSize; i++ {
		fmt.Printf("  计算交叉熵项 %d: -y_%d * log(p_%d)...\n", i, i, i)

		// 获取密文标签
		encryptedLabel := encryptedLabels[i]

		// 获取log(softmax)输出
		logSoftmax := logSoftmaxOutputs[i]

		// 计算 -log(p_i)
		negLogSoftmax, err := evaluator.MulNew(logSoftmax, -1.0)
		if err != nil {
			return nil, fmt.Errorf("计算负对数失败: %v", err)
		}

		// 与密文标签相乘：-y_i * log(p_i)
		term, err := evaluator.MulNew(negLogSoftmax, encryptedLabel)
		if err != nil {
			return nil, fmt.Errorf("计算交叉熵项失败: %v", err)
		}

		// Rescale以保持精度
		if err := evaluator.Rescale(term, term); err != nil {
			return nil, fmt.Errorf("Rescale失败: %v", err)
		}

		crossEntropyTerms[i] = term
	}

	// 步骤3: 对所有类别求和：-∑(Y_j ⊙ log(P_j))
	fmt.Println("\n📊 步骤3：对所有类别求和")

	// 初始化为第一个乘积
	sumCrossProducts := crossEntropyTerms[0].CopyNew()

	// 累加其余类别
	for j := 1; j < olwp.OutputSize; j++ {
		fmt.Printf("  累加类别%d...\n", j)
		var err error
		sumCrossProducts, err = evaluator.AddNew(sumCrossProducts, crossEntropyTerms[j])
		if err != nil {
			return nil, fmt.Errorf("累加失败(类别%d): %v", j, err)
		}
	}

	// 取负号（交叉熵损失为负对数似然）
	if err := evaluator.MulRelin(sumCrossProducts, -1.0, sumCrossProducts); err != nil {
		return nil, fmt.Errorf("取负号失败: %v", err)
	}

	fmt.Printf("  累加后层级: %d/%d\n", sumCrossProducts.Level(), params.MaxLevel())

	// 步骤4: 计算批次平均值
	fmt.Println("\n📊 步骤4：计算批次平均值")

	// 计算批次大小的倒数
	batchSize := olwp.dataPackingService.GetBatchSize()
	batchSizeInv := 1.0 / float64(batchSize)

	// 乘以批次大小的倒数
	avgLoss, err := evaluator.MulNew(sumCrossProducts, batchSizeInv)
	if err != nil {
		return nil, fmt.Errorf("计算平均值失败: %v", err)
	}

	// Rescale以保持精度
	if err := evaluator.Rescale(avgLoss, avgLoss); err != nil {
		return nil, fmt.Errorf("Rescale失败: %v", err)
	}

	fmt.Printf("✅ 密文标签交叉熵损失计算完成，最终层级: %d/%d\n", avgLoss.Level(), params.MaxLevel())
	return avgLoss, nil
}

// SetKeyManager 设置密钥管理器
func (olwp *OutputLayerWeightProcessor) SetKeyManager(keyManager *crypto.KeyManager) {
	olwp.keyManager = keyManager
}

// SetRefreshService 设置刷新服务
func (olwp *OutputLayerWeightProcessor) SetRefreshService(refreshService *crypto.RefreshService) {
	olwp.refreshService = refreshService
	// 同时设置crossEntropyLoss的刷新服务
	if olwp.crossEntropyLoss != nil {
		olwp.crossEntropyLoss.SetRefreshService(refreshService)
	}
}

// SetNetworkInfo 设置网络信息
func (olwp *OutputLayerWeightProcessor) SetNetworkInfo(onlinePeers map[int]string, myID int) {
	olwp.onlinePeers = onlinePeers
	olwp.myID = myID
	// 同时设置crossEntropyLoss的网络信息
	if olwp.crossEntropyLoss != nil {
		olwp.crossEntropyLoss.SetNetworkInfo(onlinePeers, myID)
	}
}

// GetWeights 获取权重矩阵
func (olwp *OutputLayerWeightProcessor) GetWeights() [][]float64 {
	return olwp.Weights
}

// GetBiases 获取偏置向量
func (olwp *OutputLayerWeightProcessor) GetBiases() []float64 {
	return olwp.Biases
}

// PrintWeightInfo 打印权重信息
func (olwp *OutputLayerWeightProcessor) PrintWeightInfo() {
	fmt.Printf("\n=== 输出层权重信息 ===\n")
	fmt.Printf("输入特征数: %d\n", olwp.InputSize)
	fmt.Printf("输出层神经元数: %d\n", olwp.OutputSize)

	if olwp.Weights != nil {
		fmt.Printf("权重矩阵: %d x %d\n", len(olwp.Weights), len(olwp.Weights[0]))
		fmt.Printf("偏置向量: %d\n", len(olwp.Biases))
		fmt.Printf("权重类型: 明文权重（用于同态计算）\n")
	} else {
		fmt.Printf("权重矩阵: 未初始化\n")
	}

	fmt.Printf("=====================\n\n")
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
