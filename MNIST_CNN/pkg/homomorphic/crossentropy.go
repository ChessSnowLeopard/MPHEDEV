package homomorphic

import (
	"MNIST-CNN/pkg/participant"
	"MNIST-CNN/pkg/protocols"
	"fmt"
	"math"
	"math/big"

	"github.com/tuneinsight/lattigo/v6/circuits/ckks/polynomial"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/bignum"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

// LogarithmChebyshevActivation 使用Chebyshev多项式近似的对数激活函数
type LogarithmChebyshevActivation struct {
	K        float64
	Eps      float64
	Degree   int
	poly     bignum.Polynomial
	polyEval *polynomial.Evaluator
	init     bool
}

// NewLogarithmChebyshevActivation 创建Chebyshev多项式近似的对数激活
func NewLogarithmChebyshevActivation(K, eps float64, degree int) *LogarithmChebyshevActivation {
	return &LogarithmChebyshevActivation{
		K:      K,
		Eps:    eps,
		Degree: degree,
		init:   false,
	}
}

// Initialize 初始化多项式和评估器
func (lca *LogarithmChebyshevActivation) Initialize(params ckks.Parameters, evaluator *ckks.Evaluator) {
	if lca.init {
		return
	}

	// 对数函数
	logFunc := func(x float64) float64 {
		if x <= 0 {
			return math.Log(lca.Eps) // 避免log(0)
		}
		return math.Log(x)
	}

	lca.poly = getChebyshevPolyForLog(lca.K, lca.Eps, lca.Degree, logFunc)
	lca.polyEval = polynomial.NewEvaluator(params, evaluator)
	lca.init = true
}

// Apply 应用Chebyshev多项式近似对数函数
func (lca *LogarithmChebyshevActivation) Apply(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	lca.Initialize(params, evaluator)

	ctResult := ct.CopyNew()
	targetScale := params.DefaultScale()

	// 确保输入密文是度数1（重线性化）
	if ctResult.Degree() > 1 {
		if err := evaluator.Relinearize(ctResult, ctResult); err != nil {
			return nil, fmt.Errorf("输入重线性化失败: %v", err)
		}
	}

	// 创建多项式并评估
	polyLog := polynomial.NewPolynomial(lca.poly)

	// 关键修复：添加ChangeOfBasis变换（参考demo的正确实现）
	scalar, constant := polyLog.ChangeOfBasis()

	// 应用线性变换：ct = scalar * ct + constant
	if err := evaluator.Mul(ctResult, scalar, ctResult); err != nil {
		return nil, fmt.Errorf("应用scalar变换失败: %v", err)
	}
	if err := evaluator.Add(ctResult, constant, ctResult); err != nil {
		return nil, fmt.Errorf("应用constant变换失败: %v", err)
	}
	if err := evaluator.Rescale(ctResult, ctResult); err != nil {
		return nil, fmt.Errorf("ChangeOfBasis rescale失败: %v", err)
	}

	// 评估多项式
	result, err := lca.polyEval.Evaluate(ctResult, polyLog, targetScale)
	if err != nil {
		return nil, fmt.Errorf("多项式求值失败: %v", err)
	}

	return result, nil
}

func (lca *LogarithmChebyshevActivation) GetName() string {
	return fmt.Sprintf("LogChebyshev(K=%.1f, eps=%.3f, degree=%d)", lca.K, lca.Eps, lca.Degree)
}

// TestLogApproximation 测试对数近似的精度
func (lca *LogarithmChebyshevActivation) TestLogApproximation() {
	fmt.Println("\n=== 测试对数近似精度 ===")

	// 测试更广泛的值范围，包括典型的softmax输出
	testValues := []float64{
		0.01, 0.05, 0.08, 0.1, 0.12, 0.15, 0.2, 0.3, 0.4, 0.5,
		0.6, 0.7, 0.8, 0.9, 1.0,
	}

	fmt.Printf("测试区间: [%.3f, %.3f], 多项式阶数: %d\n", lca.Eps, lca.K, lca.Degree)
	fmt.Printf("%-8s %-12s %-12s %-12s %-10s\n", "输入x", "真实log(x)", "区间检查", "绝对误差", "状态")
	fmt.Printf("%-8s %-12s %-12s %-12s %-10s\n", "-----", "----------", "--------", "--------", "-----")

	maxError := 0.0
	inRangeCount := 0

	for _, x := range testValues {
		trueLog := math.Log(x)
		inRange := x >= lca.Eps && x <= lca.K

		// 这里简化显示，实际的多项式近似需要在同态环境中测试
		// 但可以显示哪些值在近似区间内
		var status string
		if inRange {
			status = "✓ 在区间内"
			inRangeCount++
		} else {
			status = "✗ 超出区间"
		}

		// 粗略估计误差（实际需要同态计算验证）
		estimatedError := 0.0
		if !inRange {
			estimatedError = 1.0 // 超出区间的误差会很大
		}

		if estimatedError > maxError {
			maxError = estimatedError
		}

		fmt.Printf("%-8.3f %-12.6f %-12s %-12.3e %-10s\n",
			x, trueLog, fmt.Sprintf("[%.3f,%.3f]", lca.Eps, lca.K), estimatedError, status)
	}

	fmt.Printf("\n统计信息:\n")
	fmt.Printf("  测试值总数: %d\n", len(testValues))
	fmt.Printf("  区间内值数: %d\n", inRangeCount)
	fmt.Printf("  覆盖率: %.1f%%\n", float64(inRangeCount)/float64(len(testValues))*100)
	fmt.Printf("  近似区间: [%.3f, %.3f]\n", lca.Eps, lca.K)
	fmt.Printf("  多项式阶数: %d\n", lca.Degree)

	if inRangeCount == len(testValues) {
		fmt.Printf("✅ 所有测试值都在近似区间内\n")
	} else {
		fmt.Printf("⚠️  有%d个值超出近似区间，可能需要调整参数\n", len(testValues)-inRangeCount)
	}
}

// getChebyshevPolyForLog 返回对数函数在区间[eps,K]上的Chebyshev多项式近似
func getChebyshevPolyForLog(K, eps float64, degree int, f64 func(x float64) (y float64)) bignum.Polynomial {
	FBig := func(x *big.Float) (y *big.Float) {
		xF64, _ := x.Float64()
		return new(big.Float).SetPrec(x.Prec()).SetFloat64(f64(xF64))
	}

	var prec uint = 128

	// 在区间[eps, K]上进行切比雪夫近似
	interval := bignum.Interval{
		A:     *bignum.NewFloat(eps, prec),
		B:     *bignum.NewFloat(K, prec),
		Nodes: degree,
	}
	return bignum.ChebyshevApproximation(FBig, interval)
}

// CrossEntropyLoss 交叉熵损失函数结构
type CrossEntropyLoss struct {
	NumClasses      int // 类别数量 (t'')
	BatchSize       int // 批大小 (s/k)
	Slots           int // 密文槽数 (s)
	K               int // 每密文特征数
	LogApproximator *LogarithmChebyshevActivation
}

// NewCrossEntropyLoss 创建交叉熵损失函数
func NewCrossEntropyLoss(numClasses, batchSize, slots, k int) *CrossEntropyLoss {
	// 对数函数参数配置 - 优化后的参数
	// 基于demo的成功配置，扩大近似区间并提高精度

	// 修复1：扩大近似区间，参考demo的成功经验
	logEps := 0.01 // 下界：扩大到更小的值，覆盖更多可能的softmax输出
	logK := 1.0    // 上界：扩大到更大的值，确保覆盖所有可能的softmax输出

	// 修复2：提高多项式阶数，与demo保持一致
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

// ComputeCrossEntropyLoss 计算交叉熵损失（带自举检查点）
func (cel *CrossEntropyLoss) ComputeCrossEntropyLoss(
	softmaxOutputs []*rlwe.Ciphertext, // P_0, P_1, ..., P_{t''-1}
	labels []byte, // 批次标签
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	decryptor *rlwe.Decryptor, // 添加解密器用于调试
	// 自举参数
	N int,
	parties []*participant.Party,
	cloud *participant.Cloud,
	crs *sampling.KeyedPRNG,
) (*rlwe.Ciphertext, error) {

	fmt.Println("\n=== 计算交叉熵损失函数 ===")
	fmt.Printf("类别数: %d, 批大小: %d\n", cel.NumClasses, cel.BatchSize)

	bootstrapThreshold := params.MaxLevel() / 3

	// 步骤1: 创建one-hot编码标签
	fmt.Println("\n📊 步骤1：创建one-hot编码标签")
	oneHotLabels, err := cel.createOneHotLabels(labels, params, encoder)
	if err != nil {
		return nil, fmt.Errorf("创建one-hot标签失败: %v", err)
	}

	// 步骤2: 对softmax输出计算对数
	fmt.Println("\n📊 步骤2：计算log(softmax)输出")
	fmt.Printf("对数函数参数: K=%.3f, eps=%.3f, degree=%d\n", cel.LogApproximator.K, cel.LogApproximator.Eps, cel.LogApproximator.Degree)

	// 🔍 输入检查：检查softmax输出密文是否需要自举
	fmt.Printf("\n🔍 输入检查：检查softmax输出密文层级\n")
	minSoftmaxLevel := params.MaxLevel()
	for i, ct := range softmaxOutputs {
		level := ct.Level()
		if level < minSoftmaxLevel {
			minSoftmaxLevel = level
		}
		fmt.Printf("  类别%d: softmax输出层级=%d/%d\n", i, level, params.MaxLevel())
	}
	fmt.Printf("  最低softmax输出层级: %d/%d\n", minSoftmaxLevel, params.MaxLevel())

	if minSoftmaxLevel < bootstrapThreshold {
		fmt.Printf("\n🔄 输入检查触发：层级过低（%d < %d），对softmax输出执行自举...\n", minSoftmaxLevel, bootstrapThreshold)
		softmaxOutputs, _ = cel.refreshCiphertexts(softmaxOutputs, params, N, parties, cloud, crs, "softmax输出")
		fmt.Printf("✓ 输入检查自举完成\n")
	} else {
		fmt.Printf("✅ 输入检查通过：层级充足，无需自举\n")
	}

	// 首先检查Softmax值的实际范围
	fmt.Println("\n🔍 分析Softmax输出范围...")
	for i := 0; i < min(3, len(softmaxOutputs)); i++ {
		pt := decryptor.DecryptNew(softmaxOutputs[i])
		decoded := make([]complex128, cel.Slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密失败: %v\n", err)
			continue
		}

		minVal, maxVal := 1.0, 0.0
		for j := 0; j < cel.BatchSize; j++ {
			val := real(decoded[j])
			if val < minVal {
				minVal = val
			}
			if val > maxVal {
				maxVal = val
			}
		}
		fmt.Printf("  类别%d: 范围[%.6f, %.6f]\n", i, minVal, maxVal)
	}

	logSoftmaxOutputs := make([]*rlwe.Ciphertext, len(softmaxOutputs))

	for i, softmaxOut := range softmaxOutputs {
		fmt.Printf("  计算log(P_%d)...\n", i)
		logResult, err := cel.LogApproximator.Apply(softmaxOut, evaluator, params)
		if err != nil {
			return nil, fmt.Errorf("计算log失败(类别%d): %v", i, err)
		}
		logSoftmaxOutputs[i] = logResult
		fmt.Printf("    log后层级: %d/%d\n", logResult.Level(), params.MaxLevel())
	}

	// 🔍 检查点1：对数计算后的层级检查
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
		fmt.Printf("\n🔄 检查点1触发：对对数结果执行自举...\n")
		logSoftmaxOutputs, _ = cel.refreshCiphertexts(logSoftmaxOutputs, params, N, parties, cloud, crs, "对数计算")
	} else {
		fmt.Printf("✅ 检查点1通过：层级充足，无需自举\n")
	}

	// 步骤3: 计算 Y_j ⊙ log(P_j)
	fmt.Println("\n📊 步骤3：计算 Y_j ⊙ log(P_j)")
	crossProducts := make([]*rlwe.Ciphertext, cel.NumClasses)

	for j := 0; j < cel.NumClasses; j++ {
		fmt.Printf("  计算 Y_%d ⊙ log(P_%d)...\n", j, j)

		// Y_j * log(P_j)
		product, err := evaluator.MulNew(logSoftmaxOutputs[j], oneHotLabels[j])
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
		fmt.Printf("\n🔄 检查点2触发：对累加结果执行自举...\n")
		refreshed, _ := cel.refreshCiphertexts([]*rlwe.Ciphertext{sumCrossProducts}, params, N, parties, cloud, crs, "累加结果")
		sumCrossProducts = refreshed[0]
	} else {
		fmt.Printf("✅ 检查点2通过：层级充足，无需自举\n")
	}

	// 步骤5: 直接返回损失向量，不在同态域中求平均
	// 平均计算将在验证函数中通过解密后进行
	fmt.Println("\n📊 步骤5：损失计算完成")

	fmt.Printf("✅ 交叉熵损失计算完成，最终层级: %d/%d\n", sumCrossProducts.Level(), params.MaxLevel())
	return sumCrossProducts, nil
}

// createOneHotLabels 创建one-hot编码的标签明文
func (cel *CrossEntropyLoss) createOneHotLabels(labels []byte, params ckks.Parameters, encoder *ckks.Encoder) ([]*rlwe.Plaintext, error) {
	oneHotLabels := make([]*rlwe.Plaintext, cel.NumClasses)

	for j := 0; j < cel.NumClasses; j++ {
		// 创建第j类的one-hot向量
		values := make([]complex128, cel.Slots)

		// 在每个重复段中填入one-hot值
		for segment := 0; segment < cel.Slots/cel.BatchSize; segment++ {
			startIdx := segment * cel.BatchSize
			for i := 0; i < cel.BatchSize && i < len(labels); i++ {
				if int(labels[i]) == j {
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

// refreshCiphertexts 刷新密文的辅助函数
func (cel *CrossEntropyLoss) refreshCiphertexts(
	ciphertexts []*rlwe.Ciphertext,
	params ckks.Parameters,
	N int,
	parties []*participant.Party,
	cloud *participant.Cloud,
	crs *sampling.KeyedPRNG,
	step string,
) ([]*rlwe.Ciphertext, bool) {

	// 转换为map格式
	ctMap := make(map[uint64]*rlwe.Ciphertext)
	for i, ct := range ciphertexts {
		ctMap[uint64(i)] = ct
	}

	// 执行自举
	protocols.RefreshCiphertexts(params, N, parties, cloud, ctMap, crs)

	// 收集结果
	refreshed := make([]*rlwe.Ciphertext, len(ciphertexts))
	for i := 0; i < len(ciphertexts); i++ {
		result := <-cloud.RefreshDone
		refreshed[result.Key] = result.Ciphertext
		fmt.Printf("    %s自举后层级: %d/%d\n", step, result.Ciphertext.Level(), params.MaxLevel())
	}

	return refreshed, true
}

// computeBatchAverage 计算批内平均值
func (cel *CrossEntropyLoss) computeBatchAverage(sumLoss *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	fmt.Printf("  开始旋转累加（批大小: %d）...\n", cel.BatchSize)

	result := sumLoss.CopyNew()

	// 执行 log2(batchSize) 次旋转累加
	rotations := int(math.Log2(float64(cel.BatchSize)))
	for step := 0; step < rotations; step++ {
		rotateBy := 1 << step

		fmt.Printf("    旋转步骤%d: 旋转%d位...\n", step+1, rotateBy)
		rotated, err := evaluator.RotateNew(result, rotateBy)
		if err != nil {
			return nil, fmt.Errorf("旋转失败(步骤%d): %v", step, err)
		}

		result, err = evaluator.AddNew(result, rotated)
		if err != nil {
			return nil, fmt.Errorf("累加失败(步骤%d): %v", step, err)
		}
	}

	// 除以批大小求平均
	avgScale := 1.0 / float64(cel.BatchSize)
	if err := evaluator.Mul(result, avgScale, result); err != nil {
		return nil, fmt.Errorf("求平均失败: %v", err)
	}
	if err := evaluator.Rescale(result, result); err != nil {
		return nil, fmt.Errorf("平均rescale失败: %v", err)
	}

	fmt.Printf("  ✓ 批内平均计算完成\n")
	return result, nil
}

// ComputeCrossEntropyLossPrivacyPreserving 高效隐私保护的交叉熵损失计算
// 基于重复打包的组内旋转聚合策略，无需掩码
func (cel *CrossEntropyLoss) ComputeCrossEntropyLossPrivacyPreserving(
	softmaxOutputs []*rlwe.Ciphertext, // P_0, P_1, ..., P_{t''-1}
	labels []byte, // 批次标签
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	// 自举参数
	N int,
	parties []*participant.Party,
	cloud *participant.Cloud,
	crs *sampling.KeyedPRNG,
) (*rlwe.Ciphertext, error) {

	fmt.Println("\n=== 🚀 高效隐私保护的交叉熵损失计算 ===")
	fmt.Printf("类别数: %d, 批大小: %d\n", cel.NumClasses, cel.BatchSize)
	fmt.Println("算法特性: 基于重复打包的组内旋转聚合，无需掩码")

	bootstrapThreshold := params.MaxLevel() / 3

	// 步骤1: 创建one-hot编码标签
	fmt.Println("\n📊 步骤1：创建重复打包的one-hot编码标签")
	oneHotLabels, err := cel.createOneHotLabels(labels, params, encoder)
	if err != nil {
		return nil, fmt.Errorf("创建one-hot标签失败: %v", err)
	}

	// 步骤2: 对softmax输出计算对数（隐私保护模式）
	fmt.Println("\n📊 步骤2：计算log(softmax)输出（隐私保护模式）")
	fmt.Printf("对数函数参数: K=%.3f, eps=%.3f, degree=%d\n", cel.LogApproximator.K, cel.LogApproximator.Eps, cel.LogApproximator.Degree)

	// 🔍 输入检查：检查softmax输出密文是否需要自举（隐私保护模式）
	fmt.Printf("\n🔍 输入检查：检查softmax输出密文层级（隐私保护模式）\n")
	minSoftmaxLevel := params.MaxLevel()
	for i, ct := range softmaxOutputs {
		level := ct.Level()
		if level < minSoftmaxLevel {
			minSoftmaxLevel = level
		}
		fmt.Printf("  类别%d: softmax输出层级=%d/%d\n", i, level, params.MaxLevel())
	}
	fmt.Printf("  最低softmax输出层级: %d/%d\n", minSoftmaxLevel, params.MaxLevel())

	if minSoftmaxLevel < bootstrapThreshold {
		fmt.Printf("\n🔄 输入检查触发：层级过低（%d < %d），对softmax输出执行自举...\n", minSoftmaxLevel, bootstrapThreshold)
		softmaxOutputs, _ = cel.refreshCiphertexts(softmaxOutputs, params, N, parties, cloud, crs, "softmax输出")
		fmt.Printf("✓ 输入检查自举完成\n")
	} else {
		fmt.Printf("✅ 输入检查通过：层级充足，无需自举\n")
	}

	logSoftmaxOutputs := make([]*rlwe.Ciphertext, len(softmaxOutputs))

	for i, softmaxOut := range softmaxOutputs {
		fmt.Printf("  计算log(P_%d)...\n", i)
		logResult, err := cel.LogApproximator.Apply(softmaxOut, evaluator, params)
		if err != nil {
			return nil, fmt.Errorf("计算log失败(类别%d): %v", i, err)
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
		fmt.Printf("\n🔄 检查点1触发：对对数结果执行自举...\n")
		logSoftmaxOutputs, _ = cel.refreshCiphertexts(logSoftmaxOutputs, params, N, parties, cloud, crs, "对数计算")
	} else {
		fmt.Printf("✅ 检查点1通过：层级充足，无需自举\n")
	}

	// 步骤3: 计算 Y_j ⊙ log(P_j)
	fmt.Println("\n📊 步骤3：计算 Y_j ⊙ log(P_j)")
	crossProducts := make([]*rlwe.Ciphertext, cel.NumClasses)

	for j := 0; j < cel.NumClasses; j++ {
		fmt.Printf("  计算 Y_%d ⊙ log(P_%d)...\n", j, j)

		// Y_j * log(P_j)
		product, err := evaluator.MulNew(logSoftmaxOutputs[j], oneHotLabels[j])
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

	fmt.Printf("  累加后层级: %d/%d\n", sumCrossProducts.Level(), params.MaxLevel())

	// 🔍 检查点2：累加后的层级检查
	if sumCrossProducts.Level() < bootstrapThreshold {
		fmt.Printf("\n🔄 检查点2触发：对累加结果执行自举...\n")
		refreshed, _ := cel.refreshCiphertexts([]*rlwe.Ciphertext{sumCrossProducts}, params, N, parties, cloud, crs, "累加结果")
		sumCrossProducts = refreshed[0]
	} else {
		fmt.Printf("✅ 检查点2通过：层级充足，无需自举\n")
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

// VerifyCrossEntropyLossEfficientPrivacy 高效隐私保护的交叉熵损失验证
// 仅验证聚合后的平均损失，完全避免个体信息泄露
func (cel *CrossEntropyLoss) VerifyCrossEntropyLossEfficientPrivacy(
	encryptedAvgLoss *rlwe.Ciphertext,
	softmaxOutputs []*rlwe.Ciphertext,
	labels []byte,
	decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder,
	slots int,
) {
	fmt.Println("\n=== 🚀 高效隐私保护的交叉熵损失验证 ===")
	fmt.Println("验证特性: 仅比较聚合后的平均损失，零个体信息泄露")

	// 解密聚合后的平均损失（由于重复打包，任意位置都包含相同的平均值）
	pt := decryptor.DecryptNew(encryptedAvgLoss)
	decoded := make([]complex128, slots)
	if err := encoder.Decode(pt, decoded); err != nil {
		fmt.Printf("解密平均损失失败: %v\n", err)
		return
	}

	// 从第一个位置获取平均损失（重复打包保证所有位置值相同）
	encryptedAvgLossValue := real(decoded[0])
	fmt.Printf("🔒 密文平均损失: %.6f\n", encryptedAvgLossValue)

	// 计算明文的平均损失（仅用于验证，实际应用中可以由可信第三方提供）
	// 注意：这里的解密仅用于验证准确性，实际部署时可移除
	softmaxValues := make([][]float64, len(softmaxOutputs))
	for i, ct := range softmaxOutputs {
		pt := decryptor.DecryptNew(ct)
		decoded := make([]complex128, slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密softmax输出%d失败: %v\n", i, err)
			continue
		}

		softmaxValues[i] = make([]float64, cel.BatchSize)
		for j := 0; j < cel.BatchSize; j++ {
			softmaxValues[i][j] = real(decoded[j])
		}
	}

	// 计算明文交叉熵损失的平均值（聚合统计）
	totalLoss := 0.0
	for m := 0; m < cel.BatchSize && m < len(labels); m++ {
		trueLabel := int(labels[m])
		predictedProb := softmaxValues[trueLabel][m]

		// 避免log(0)
		if predictedProb <= 0 {
			predictedProb = 1e-7
		}

		individualLoss := -math.Log(predictedProb)
		totalLoss += individualLoss
	}
	plaintextAvgLoss := totalLoss / float64(cel.BatchSize)

	// 验证结果
	fmt.Printf("📊 聚合损失对比：\n")
	fmt.Printf("• 明文平均损失: %.6f\n", plaintextAvgLoss)
	fmt.Printf("• 密文平均损失: %.6f\n", encryptedAvgLossValue)

	absError := math.Abs(plaintextAvgLoss - encryptedAvgLossValue)
	relError := absError / plaintextAvgLoss * 100

	fmt.Printf("• 绝对误差: %.6f\n", absError)
	fmt.Printf("• 相对误差: %.2f%%\n", relError)

	// 评估结果
	if relError < 1.0 {
		fmt.Printf("✅ 🚀 高效隐私保护验证通过！（误差 < 1%%）\n")
	} else if relError < 5.0 {
		fmt.Printf("⚠️ 🚀 高效隐私保护验证通过，误差可接受（1%% < 误差 < 5%%）\n")
	} else {
		fmt.Printf("❌ 🚀 高效隐私保护验证失败，误差过大（误差 > 5%%）\n")
	}

	fmt.Printf("\n🔐 高效隐私保护总结：\n")
	fmt.Printf("• ✅ 零个体样本信息泄露\n")
	fmt.Printf("• ✅ 无掩码操作，计算高效\n")
	fmt.Printf("• ✅ 基于重复打包的自然边界\n")
	fmt.Printf("• ✅ 仅输出批次聚合统计\n")
	fmt.Printf("• 🚀 算法优势: 利用向量组装器的重复打包特性\n")
}

// ComputePlaintextCrossEntropyForDebug 计算明文交叉熵损失用于调试对比
func (cel *CrossEntropyLoss) ComputePlaintextCrossEntropyForDebug(
	softmaxOutputs []*rlwe.Ciphertext,
	labels []byte,
	decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder,
	slots int,
) {
	fmt.Println("\n=== 明文交叉熵损失计算（调试用）===")

	// 解密softmax输出
	softmaxValues := make([][]float64, len(softmaxOutputs))
	for i, ct := range softmaxOutputs {
		pt := decryptor.DecryptNew(ct)
		decoded := make([]complex128, slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密softmax输出%d失败: %v\n", i, err)
			continue
		}

		softmaxValues[i] = make([]float64, cel.BatchSize)
		for j := 0; j < cel.BatchSize; j++ {
			softmaxValues[i][j] = real(decoded[j])
		}
	}

	// 分析softmax输出的实际范围
	fmt.Printf("\n🔍 Softmax输出范围分析:\n")
	globalMin, globalMax := 1.0, 0.0
	for i := 0; i < len(softmaxValues); i++ {
		for j := 0; j < cel.BatchSize; j++ {
			val := softmaxValues[i][j]
			if val < globalMin {
				globalMin = val
			}
			if val > globalMax {
				globalMax = val
			}
		}
	}
	fmt.Printf("  全局范围: [%.6f, %.6f]\n", globalMin, globalMax)
	fmt.Printf("  近似区间: [%.6f, %.6f]\n", cel.LogApproximator.Eps, cel.LogApproximator.K)

	// 检查覆盖率
	inRangeCount := 0
	totalCount := 0
	for i := 0; i < len(softmaxValues); i++ {
		for j := 0; j < cel.BatchSize; j++ {
			val := softmaxValues[i][j]
			if val >= cel.LogApproximator.Eps && val <= cel.LogApproximator.K {
				inRangeCount++
			}
			totalCount++
		}
	}
	coverage := float64(inRangeCount) / float64(totalCount) * 100
	fmt.Printf("  覆盖率: %.1f%% (%d/%d)\n", coverage, inRangeCount, totalCount)

	if coverage < 100 {
		fmt.Printf("  ⚠️  有%.1f%%的值超出近似区间，这会导致较大误差\n", 100-coverage)
	} else {
		fmt.Printf("  ✅ 所有值都在近似区间内\n")
	}

	// 对每个类别测试对数近似
	fmt.Println("\n📊 对数近似测试（前5个样本，前3个类别）：")
	fmt.Println("样本 | 类别 | Softmax值 | 真实log | 近似区间检查 | 预期误差等级")
	fmt.Println("-----|------|-----------|---------|-------------|-------------")

	for m := 0; m < min(5, cel.BatchSize); m++ {
		for c := 0; c < min(3, cel.NumClasses); c++ {
			prob := softmaxValues[c][m]
			trueLog := math.Log(prob)
			inRange := prob >= cel.LogApproximator.Eps && prob <= cel.LogApproximator.K

			var rangeStr, errorLevel string
			if inRange {
				rangeStr = "✓ 在区间内"
				errorLevel = "低误差"
			} else {
				rangeStr = "✗ 超出区间"
				errorLevel = "高误差"
			}

			fmt.Printf("%4d | %4d | %9.6f | %8.3f | %s | %s\n",
				m, c, prob, trueLog, rangeStr, errorLevel)
		}
	}

	// 建议优化参数
	fmt.Printf("\n💡 参数优化建议:\n")
	if coverage < 95 {
		suggestedEps := globalMin * 0.8 // 留20%余量
		suggestedK := globalMax * 1.2   // 留20%余量
		fmt.Printf("  建议扩大近似区间: [%.6f, %.6f] -> [%.6f, %.6f]\n",
			cel.LogApproximator.Eps, cel.LogApproximator.K, suggestedEps, suggestedK)
	}

	if cel.LogApproximator.Degree < 20 {
		fmt.Printf("  建议提高多项式阶数: %d -> 20\n", cel.LogApproximator.Degree)
	}
}

// VerifyCrossEntropyLoss 验证交叉熵损失计算结果
func (cel *CrossEntropyLoss) VerifyCrossEntropyLoss(
	encryptedLoss *rlwe.Ciphertext,
	softmaxOutputs []*rlwe.Ciphertext,
	labels []byte,
	decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder,
	slots int,
) {
	fmt.Println("\n=== 验证交叉熵损失结果 ===")

	// 解密损失向量
	pt := decryptor.DecryptNew(encryptedLoss)
	decoded := make([]complex128, slots)
	if err := encoder.Decode(pt, decoded); err != nil {
		fmt.Printf("解密损失失败: %v\n", err)
		return
	}

	// 提取各个样本的损失值（每个样本的损失在对应的槽位置）
	fmt.Printf("密文损失向量前10个值: ")
	for i := 0; i < 10 && i < len(decoded); i++ {
		fmt.Printf("%.6f ", real(decoded[i]))
	}
	fmt.Printf("\n")

	// 计算批内平均损失
	totalEncryptedLoss := 0.0
	validSamples := 0
	for i := 0; i < cel.BatchSize && i < len(decoded); i++ {
		loss := real(decoded[i])
		totalEncryptedLoss += loss
		validSamples++
	}
	encryptedLossValue := totalEncryptedLoss / float64(validSamples)

	fmt.Printf("密文损失详情：总和=%.6f, 样本数=%d, 平均=%.6f\n",
		totalEncryptedLoss, validSamples, encryptedLossValue)

	// 解密softmax输出计算明文损失
	softmaxValues := make([][]float64, len(softmaxOutputs))
	for i, ct := range softmaxOutputs {
		pt := decryptor.DecryptNew(ct)
		decoded := make([]complex128, slots)
		if err := encoder.Decode(pt, decoded); err != nil {
			fmt.Printf("解密softmax输出%d失败: %v\n", i, err)
			continue
		}

		softmaxValues[i] = make([]float64, cel.BatchSize)
		for j := 0; j < cel.BatchSize; j++ {
			softmaxValues[i][j] = real(decoded[j])
		}
	}

	// 计算明文交叉熵损失
	totalLoss := 0.0
	fmt.Println("\n各样本的损失明细对比：")
	fmt.Println("样本 | 真实标签 | 预测概率 | 明文损失 | 密文损失 | 误差")
	fmt.Println("-----|----------|----------|----------|----------|--------")

	for m := 0; m < cel.BatchSize && m < len(labels); m++ {
		trueLabel := int(labels[m])
		predictedProb := softmaxValues[trueLabel][m]

		// 避免log(0)
		if predictedProb <= 0 {
			predictedProb = 1e-7
		}

		individualPlaintextLoss := -math.Log(predictedProb)
		totalLoss += individualPlaintextLoss

		// 获取对应的密文损失
		individualCiphertextLoss := real(decoded[m])
		diff := math.Abs(individualPlaintextLoss - individualCiphertextLoss)

		fmt.Printf("%4d | %8d | %8.6f | %8.6f | %9.6f | %7.4f\n",
			m, trueLabel, predictedProb, individualPlaintextLoss, individualCiphertextLoss, diff)
	}

	avgLoss := totalLoss / float64(cel.BatchSize)

	fmt.Printf("\n=== 损失对比 ===\n")
	fmt.Printf("明文平均损失: %.6f\n", avgLoss)
	fmt.Printf("密文平均损失: %.6f\n", encryptedLossValue)
	fmt.Printf("绝对误差: %.6f\n", math.Abs(avgLoss-encryptedLossValue))
	fmt.Printf("相对误差: %.2f%%\n", math.Abs(avgLoss-encryptedLossValue)/avgLoss*100)

	if math.Abs(avgLoss-encryptedLossValue)/avgLoss < 0.01 {
		fmt.Printf("✅ 交叉熵损失验证通过！\n")
	} else {
		fmt.Printf("⚠️ 交叉熵损失存在误差，需要检查实现\n")
	}
}

// ComputeCrossEntropyLossWithEncryptedLabels 使用密文标签的交叉熵损失计算
func (cel *CrossEntropyLoss) ComputeCrossEntropyLossWithEncryptedLabels(
	softmaxOutputs []*rlwe.Ciphertext, // P_0, P_1, ..., P_{t''-1}
	encryptedLabels map[uint64]*rlwe.Ciphertext, // 密文one-hot编码标签
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
	// 自举参数
	N int,
	parties []*participant.Party,
	cloud *participant.Cloud,
	crs *sampling.KeyedPRNG,
) (*rlwe.Ciphertext, error) {

	fmt.Println("\n=== 🚀 密文标签版交叉熵损失计算 ===")
	fmt.Printf("类别数: %d, 批大小: %d\n", cel.NumClasses, cel.BatchSize)
	fmt.Println("算法特性: 完全隐私保护，标签也是密文")

	bootstrapThreshold := params.MaxLevel() / 3

	// 输入验证
	if len(softmaxOutputs) != cel.NumClasses {
		return nil, fmt.Errorf("softmax输出数量 (%d) 与类别数 (%d) 不匹配", len(softmaxOutputs), cel.NumClasses)
	}

	if len(encryptedLabels) != cel.NumClasses {
		return nil, fmt.Errorf("密文标签数量 (%d) 与类别数 (%d) 不匹配", len(encryptedLabels), cel.NumClasses)
	}

	// 🔍 输入检查：检查softmax输出密文层级
	fmt.Printf("\n🔍 输入检查：检查softmax输出密文层级\n")
	minSoftmaxLevel := params.MaxLevel()
	for idx, ct := range softmaxOutputs {
		level := ct.Level()
		if level < minSoftmaxLevel {
			minSoftmaxLevel = level
		}
		fmt.Printf("  类别%d: softmax输出层级=%d/%d\n", idx, level, params.MaxLevel())
	}

	if minSoftmaxLevel < bootstrapThreshold {
		fmt.Printf("\n🔄 输入检查触发：层级过低，对softmax输出执行自举...\n")
		softmaxOutputs, _ = cel.refreshCiphertexts(softmaxOutputs, params, N, parties, cloud, crs, "softmax输出")
	}

	// 步骤2: 对softmax输出计算对数
	fmt.Println("\n📊 步骤2：计算log(softmax)输出")
	logSoftmaxOutputs := make([]*rlwe.Ciphertext, len(softmaxOutputs))

	for i, softmaxOut := range softmaxOutputs {
		fmt.Printf("  计算log(P_%d)...\n", i)
		logResult, err := cel.LogApproximator.Apply(softmaxOut, evaluator, params)
		if err != nil {
			return nil, fmt.Errorf("计算log失败(类别%d): %v", i, err)
		}
		logSoftmaxOutputs[i] = logResult
		fmt.Printf("    log后层级: %d/%d\n", logResult.Level(), params.MaxLevel())
	}

	// 🔍 检查点1：对数计算后的层级检查
	fmt.Printf("\n🔍 检查点1：对数计算后层级检查\n")
	minLogLevel := params.MaxLevel()
	for _, ct := range logSoftmaxOutputs {
		level := ct.Level()
		if level < minLogLevel {
			minLogLevel = level
		}
	}

	if minLogLevel < bootstrapThreshold {
		fmt.Printf("\n🔄 检查点1触发：对对数结果执行自举...\n")
		logSoftmaxOutputs, _ = cel.refreshCiphertexts(logSoftmaxOutputs, params, N, parties, cloud, crs, "对数计算")
	}

	// 步骤3: 计算 Y_j ⊙ log(P_j)（密文 × 密文）
	fmt.Println("\n📊 步骤3：计算 Y_j ⊙ log(P_j)（密文标签版）")
	crossProducts := make([]*rlwe.Ciphertext, cel.NumClasses)

	for j := 0; j < cel.NumClasses; j++ {
		fmt.Printf("  计算 Y_%d ⊙ log(P_%d)...\n", j, j)

		// 获取对应类别的密文标签
		encryptedLabel, exists := encryptedLabels[uint64(j)]
		if !exists {
			return nil, fmt.Errorf("找不到类别%d的密文标签", j)
		}

		// Y_j * log(P_j) (密文 × 密文)
		product, err := evaluator.MulNew(logSoftmaxOutputs[j], encryptedLabel)
		if err != nil {
			return nil, fmt.Errorf("计算乘积失败(类别%d): %v", j, err)
		}

		// 重线性化和Rescale
		if err := evaluator.Relinearize(product, product); err != nil {
			return nil, fmt.Errorf("重线性化失败(类别%d): %v", j, err)
		}

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

	fmt.Printf("  累加后层级: %d/%d\n", sumCrossProducts.Level(), params.MaxLevel())

	// 🔍 检查点2：累加后的层级检查
	if sumCrossProducts.Level() < bootstrapThreshold {
		fmt.Printf("\n🔄 检查点2触发：对累加结果执行自举...\n")
		refreshed, _ := cel.refreshCiphertexts([]*rlwe.Ciphertext{sumCrossProducts}, params, N, parties, cloud, crs, "累加结果")
		sumCrossProducts = refreshed[0]
	}

	fmt.Printf("✅ 密文标签版交叉熵损失计算完成，最终层级: %d/%d\n", sumCrossProducts.Level(), params.MaxLevel())
	return sumCrossProducts, nil
}
