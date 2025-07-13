package activation

import (
	"MPHEDev/pkg/core/participant/crypto"
	"MPHEDev/pkg/core/participant/interfaces"
	"fmt"
	"math"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// CrossEntropyLoss 交叉熵损失函数结构
type CrossEntropyLoss struct {
	NumClasses      int // 类别数量
	BatchSize       int // 批大小
	Slots           int // 密文槽数
	K               int // 每密文特征数
	LogApproximator interfaces.ActivationFunction
	RefreshService  *crypto.RefreshService // 刷新服务
	OnlinePeers     map[int]string         // 在线参与方信息
	MyID            int                    // 当前参与方ID
}

// NewCrossEntropyLoss 创建交叉熵损失函数
func NewCrossEntropyLoss(numClasses, batchSize, slots, k int) *CrossEntropyLoss {
	// 对数函数参数配置 - 优化后的参数
	logEps := 0.01  // 下界：扩大到更小的值，覆盖更多可能的softmax输出
	logK := 1.0     // 上界：扩大到更大的值，确保覆盖所有可能的softmax输出
	logDegree := 20 // 高阶多项式，提高近似精度

	fmt.Printf("🔧 对数近似参数优化:\n")
	fmt.Printf("  近似区间: [%.3f, %.3f] (扩大了区间范围)\n", logEps, logK)
	fmt.Printf("  多项式阶数: %d (提高了精度)\n", logDegree)

	return &CrossEntropyLoss{
		NumClasses:      numClasses,
		BatchSize:       batchSize,
		Slots:           slots,
		K:               k,
		LogApproximator: NewLogarithmChebyshevActivation(logK, logEps, logDegree),
	}
}

// ComputeLoss 计算交叉熵损失（与现有接口兼容）
func (cel *CrossEntropyLoss) ComputeLoss(
	softmaxOutputs []*rlwe.Ciphertext, // P_0, P_1, ..., P_{t''-1}
	labels []int, // 批次标签
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder) (*rlwe.Ciphertext, error) {
	return cel.ComputeCrossEntropyLoss(softmaxOutputs, labels, params, evaluator, encoder)
}

// ComputeCrossEntropyLoss 计算交叉熵损失（与MNIST_CNN的ComputeCrossEntropyLossPrivacyPreserving保持一致）
func (cel *CrossEntropyLoss) ComputeCrossEntropyLoss(
	softmaxOutputs []*rlwe.Ciphertext, // P_0, P_1, ..., P_{t''-1}
	labels []int, // 批次标签
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
) (*rlwe.Ciphertext, error) {

	fmt.Println("\n=== 🚀 高效隐私保护的交叉熵损失计算 ===")
	fmt.Printf("类别数: %d, 批大小: %d\n", cel.NumClasses, cel.BatchSize)
	fmt.Println("算法特性: 基于重复打包的组内旋转聚合，无需掩码")

	bootstrapThreshold := params.MaxLevel() / 3

	// 检查输入密文级别
	if err := cel.checkAndRefreshInputs(softmaxOutputs, params, "交叉熵输入"); err != nil {
		return nil, err
	}

	// 步骤1: 创建one-hot编码标签
	fmt.Println("\n📊 步骤1：创建重复打包的one-hot编码标签")
	oneHotLabels, err := cel.createOneHotLabels(labels, params, encoder)
	if err != nil {
		return nil, fmt.Errorf("创建one-hot标签失败: %v", err)
	}

	// 步骤2: 对softmax输出计算对数（隐私保护模式）
	fmt.Println("\n📊 步骤2：计算log(softmax)输出（隐私保护模式）")
	fmt.Printf("对数函数参数: %s\n", cel.LogApproximator.GetName())

	logSoftmaxOutputs := make([]*rlwe.Ciphertext, len(softmaxOutputs))
	for i, softmaxOutput := range softmaxOutputs {
		fmt.Printf("  计算log(softmax_%d)...\n", i)
		logResult, err := cel.LogApproximator.Apply(softmaxOutput, evaluator, params)
		if err != nil {
			return nil, fmt.Errorf("计算log(softmax_%d)失败: %v", i, err)
		}
		logSoftmaxOutputs[i] = logResult
		fmt.Printf("    log后层级: %d/%d\n", logResult.Level(), params.MaxLevel())
	}

	// 🔍 检查点1：对数计算后的层级检查（无解密）
	fmt.Printf("\n🔍 检查点1：对数计算后层级检查\n")
	minLogLevel := params.MaxLevel()
	for i, ct := range logSoftmaxOutputs {
		level := ct.Level()
		if level < minLogLevel {
			minLogLevel = level
		}
		fmt.Printf("  类别%d: log后层级 %d/%d\n", i, level, params.MaxLevel())
	}

	if minLogLevel < bootstrapThreshold {
		fmt.Printf("\n🔄 检查点1触发：对对数结果执行刷新...\n")
		if err := cel.checkAndRefreshInputs(logSoftmaxOutputs, params, "对数计算"); err != nil {
			return nil, err
		}
	} else {
		fmt.Printf("✅ 检查点1通过：层级充足，无需刷新\n")
	}

	// 步骤3: 计算 Y_j ⊙ log(P_j) （与MNIST_CNN保持一致）
	fmt.Println("\n📊 步骤3：计算 Y_j ⊙ log(P_j)")
	crossProducts := make([]*rlwe.Ciphertext, cel.NumClasses)

	for j := 0; j < cel.NumClasses; j++ {
		fmt.Printf("  计算 Y_%d ⊙ log(P_%d)...\n", j, j)

		// 获取one-hot标签
		oneHotLabel := oneHotLabels[j]

		// 获取log(softmax)输出
		logSoftmax := logSoftmaxOutputs[j]

		// Y_j * log(P_j) （与MNIST_CNN保持一致）
		product, err := evaluator.MulNew(logSoftmax, oneHotLabel)
		if err != nil {
			return nil, fmt.Errorf("计算乘积失败(类别%d): %v", j, err)
		}

		// Rescale
		if err := evaluator.Rescale(product, product); err != nil {
			return nil, fmt.Errorf("Rescale失败(类别%d): %v", j, err)
		}

		crossProducts[j] = product
		fmt.Printf("    乘积后层级: %d/%d\n", product.Level(), params.MaxLevel())
	}

	// 步骤4: 对所有类别求和：-∑(Y_j ⊙ log(P_j))
	fmt.Println("\n📊 步骤4：对所有类别求和")

	// 初始化为第一个乘积
	sumCrossProducts := crossProducts[0].CopyNew()

	// 累加其余类别
	for j := 1; j < cel.NumClasses; j++ {
		fmt.Printf("  累加类别%d...\n", j)
		var err error
		sumCrossProducts, err = evaluator.AddNew(sumCrossProducts, crossProducts[j])
		if err != nil {
			return nil, fmt.Errorf("累加失败(类别%d): %v", j, err)
		}
	}

	// 取负号（交叉熵损失为负对数似然）
	if err := evaluator.MulRelin(sumCrossProducts, -1.0, sumCrossProducts); err != nil {
		return nil, fmt.Errorf("取负号失败: %v", err)
	}
	// MulRelin不会改变scale，不需要rescale

	fmt.Printf("  累加后层级: %d/%d\n", sumCrossProducts.Level(), params.MaxLevel())

	// 🔍 检查点2：累加后的层级检查
	if sumCrossProducts.Level() < bootstrapThreshold {
		fmt.Printf("\n🔄 检查点2触发：对累加结果执行刷新...\n")
		if err := cel.checkAndRefreshInputs([]*rlwe.Ciphertext{sumCrossProducts}, params, "累加结果"); err != nil {
			return nil, err
		}
	} else {
		fmt.Printf("✅ 检查点2通过：层级充足，无需刷新\n")
	}

	// 步骤5: 🚀 高效隐私保护的批内平均计算（基于重复打包的组内旋转）
	fmt.Println("\n📊 步骤5：🚀 高效组内旋转聚合")
	avgLoss, err := cel.computeBatchAverageEfficientPrivacy(sumCrossProducts, evaluator, params)
	if err != nil {
		return nil, fmt.Errorf("高效聚合失败: %v", err)
	}

	fmt.Printf("✅ 🚀 高效隐私保护的交叉熵损失计算完成，最终层级: %d/%d\n", avgLoss.Level(), params.MaxLevel())
	return avgLoss, nil
}

// createOneHotLabels 创建one-hot编码标签明文（与MNIST_CNN保持一致）
func (cel *CrossEntropyLoss) createOneHotLabels(labels []int, params ckks.Parameters, encoder *ckks.Encoder) ([]*rlwe.Plaintext, error) {
	oneHotLabels := make([]*rlwe.Plaintext, cel.NumClasses)

	for j := 0; j < cel.NumClasses; j++ {
		// 创建第j类的one-hot向量
		values := make([]complex128, cel.Slots)

		// 在每个重复段中填入one-hot值（与MNIST_CNN保持一致）
		for segment := 0; segment < cel.Slots/cel.BatchSize; segment++ {
			startIdx := segment * cel.BatchSize
			for i := 0; i < cel.BatchSize && i < len(labels); i++ {
				if labels[i] == j {
					values[startIdx+i] = complex(1.0, 0)
				} else {
					values[startIdx+i] = complex(0.0, 0)
				}
			}
		}

		// 编码为明文
		pt := ckks.NewPlaintext(params, params.MaxLevel())
		if err := encoder.Encode(values, pt); err != nil {
			return nil, fmt.Errorf("编码one-hot标签失败(类别%d): %v", j, err)
		}

		oneHotLabels[j] = pt
		fmt.Printf("  创建类别%d的one-hot标签\n", j)
	}

	return oneHotLabels, nil
}

// SetRefreshService 设置刷新服务
func (cel *CrossEntropyLoss) SetRefreshService(refreshService *crypto.RefreshService) {
	cel.RefreshService = refreshService
}

// SetNetworkInfo 设置网络信息
func (cel *CrossEntropyLoss) SetNetworkInfo(onlinePeers map[int]string, myID int) {
	cel.OnlinePeers = onlinePeers
	cel.MyID = myID
}

// checkAndRefreshInputs 检查并刷新输入密文
func (cel *CrossEntropyLoss) checkAndRefreshInputs(
	ciphertexts []*rlwe.Ciphertext,
	params ckks.Parameters,
	stepName string,
) error {
	if cel.RefreshService == nil {
		// 如果没有刷新服务，直接返回
		return nil
	}

	// 设置刷新阈值
	bootstrapThreshold := params.MaxLevel() / 3

	// 检查最低层级
	minLevel := params.MaxLevel()
	for _, ct := range ciphertexts {
		if ct.Level() < minLevel {
			minLevel = ct.Level()
		}
	}

	fmt.Printf("\n🔍 %s层级检查: 最低层级=%d/%d, 阈值=%d\n",
		stepName, minLevel, params.MaxLevel(), bootstrapThreshold)

	if minLevel < bootstrapThreshold {
		fmt.Printf("🔄 触发刷新: 层级过低（%d < %d）\n", minLevel, bootstrapThreshold)

		// 调用刷新服务
		if cel.RefreshService != nil {
			if len(cel.OnlinePeers) > 0 && cel.MyID >= 0 {
				fmt.Printf("🔄 执行同步刷新...\n")
				refreshedCiphertexts, err := cel.RefreshService.RefreshCiphertextsSync(
					ciphertexts, cel.OnlinePeers, cel.MyID, fmt.Sprintf("%s_refresh", stepName))
				if err != nil {
					return fmt.Errorf("刷新失败: %v", err)
				}

				// 更新原始密文数组
				for i, refreshed := range refreshedCiphertexts {
					if i < len(ciphertexts) {
						ciphertexts[i] = refreshed
					}
				}

				fmt.Printf("✅ 刷新完成，层级已恢复\n")
			} else {
				fmt.Printf("⚠️  网络信息未设置，跳过刷新\n")
			}
		} else {
			fmt.Printf("⚠️  刷新服务未设置，跳过刷新\n")
		}
	} else {
		fmt.Printf("✅ 层级充足，无需刷新\n")
	}

	return nil
}

// ValidateInputs 验证输入参数
func (cel *CrossEntropyLoss) ValidateInputs(softmaxOutputs []*rlwe.Ciphertext, labels []int) error {
	if len(softmaxOutputs) == 0 {
		return fmt.Errorf("softmax outputs cannot be empty")
	}
	if len(labels) == 0 {
		return fmt.Errorf("labels cannot be empty")
	}
	if len(softmaxOutputs) != cel.NumClasses {
		return fmt.Errorf("softmax outputs count (%d) != num classes (%d)", len(softmaxOutputs), cel.NumClasses)
	}

	// 检查标签值是否在有效范围内
	for i, label := range labels {
		if label < 0 || label >= cel.NumClasses {
			return fmt.Errorf("label[%d] = %d is not in [0,%d) range", i, label, cel.NumClasses)
		}
	}

	return nil
}

// GetCiphertextLevel 获取密文级别
func (cel *CrossEntropyLoss) GetCiphertextLevel(ciphertext *rlwe.Ciphertext) int {
	if ciphertext == nil {
		return -1
	}
	return ciphertext.Level()
}

// EstimateLevelConsumption 估算操作消耗的密文级别
func (cel *CrossEntropyLoss) EstimateLevelConsumption() int {
	// 交叉熵损失计算的大致级别消耗：
	// - 对数计算: 约2级
	// - 乘法操作: 约1级
	// - 求和操作: 约1级
	// - 平均值计算: 约1级
	return 5
}

func (cel *CrossEntropyLoss) GetName() string {
	return fmt.Sprintf("CrossEntropyLoss(classes=%d, batch=%d, log=%s)",
		cel.NumClasses, cel.BatchSize, cel.LogApproximator.GetName())
}

// computeBatchAverageEfficientPrivacy 高效隐私保护的批内平均计算
// 基于重复打包的组内旋转聚合策略，完全无需掩码
func (cel *CrossEntropyLoss) computeBatchAverageEfficientPrivacy(sumLoss *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	fmt.Printf("  🚀 开始高效组内旋转聚合（批大小: %d）...\n", cel.BatchSize)
	fmt.Printf("  输入格式: [L₀,L₁,...,L_{%d} | L₀,L₁,...,L_{%d} | ... ]\n", cel.BatchSize-1, cel.BatchSize-1)

	result := sumLoss.CopyNew()

	// 🔄 组内旋转聚合：基于重复打包的自然边界
	// 由于每组都是相同的损失值序列，旋转不会跨组污染
	rotations := int(math.Log2(float64(cel.BatchSize)))
	fmt.Printf("  需要 %d 轮组内旋转 (log₂ %d)\n", rotations, cel.BatchSize)

	for step := 0; step < rotations; step++ {
		rotateBy := 1 << step // 1, 2, 4, 8, ...

		fmt.Printf("    🔄 轮次%d: 组内旋转%d位\n", step+1, rotateBy)

		// 组内旋转：利用重复打包模式的自然边界
		rotated, err := evaluator.RotateNew(result, rotateBy)
		if err != nil {
			return nil, fmt.Errorf("组内旋转失败(轮次%d): %v", step+1, err)
		}

		// 聚合：每组内的损失值累加
		result, err = evaluator.AddNew(result, rotated)
		if err != nil {
			return nil, fmt.Errorf("聚合失败(轮次%d): %v", step+1, err)
		}

		fmt.Printf("    ✓ 轮次%d完成\n", step+1)
	}

	// 🧮 计算平均值：除以批大小
	avgScale := 1.0 / float64(cel.BatchSize)
	fmt.Printf("  📊 平均化: 每个聚合值除以批大小 %d\n", cel.BatchSize)

	if err := evaluator.Mul(result, avgScale, result); err != nil {
		return nil, fmt.Errorf("平均化失败: %v", err)
	}
	if err := evaluator.Rescale(result, result); err != nil {
		return nil, fmt.Errorf("rescale失败: %v", err)
	}

	fmt.Printf("  ✅ 高效聚合完成！\n")
	fmt.Printf("  结果格式: [avg_loss, avg_loss, ..., avg_loss | 重复组 | ...]\n")
	fmt.Printf("  🔒 隐私特性: 个体样本损失完全隐藏，仅保留聚合统计\n")
	fmt.Printf("  ⚡ 效率特性: 无掩码操作，利用重复打包的自然特性\n")

	return result, nil
}
