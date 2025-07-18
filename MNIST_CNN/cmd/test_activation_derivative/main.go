package main

import (
	"MNIST-CNN/pkg/homomorphic"
	"fmt"
	"math"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("同态加密激活函数导数测试")
	fmt.Println("========================================")

	// 参数设置
	params, err := ckks.NewParametersFromLiteral(
		ckks.ParametersLiteral{
			LogN:            14,
			LogQ:            []int{55, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45},
			LogP:            []int{61},
			LogDefaultScale: 45,
		})
	if err != nil {
		panic(err)
	}

	// 密钥生成
	kgen := rlwe.NewKeyGenerator(params)
	sk := kgen.GenSecretKeyNew()
	encoder := ckks.NewEncoder(params)
	encryptor := rlwe.NewEncryptor(params, sk)
	decryptor := rlwe.NewDecryptor(params, sk)

	// 重线性化密钥
	rlk := kgen.GenRelinearizationKeyNew(sk)
	evk := rlwe.NewMemEvaluationKeySet(rlk)
	evaluator := ckks.NewEvaluator(params, evk)

	fmt.Printf("参数详情: LogN=%d, 最大层数=%d, 槽数=%d\n",
		params.LogN(), params.MaxLevel(), params.MaxSlots())

	// 测试数据 - 使用与demo相同的测试值
	testValues := []float64{-5, -3, -2, -1, -0.5, 0, 0.5, 1, 2, 3, 5}

	// 创建输入数据
	plaintext := ckks.NewPlaintext(params, params.MaxLevel())
	values := make([]float64, plaintext.Slots())

	// 填充测试值
	for i := 0; i < len(testValues) && i < plaintext.Slots(); i++ {
		values[i] = testValues[i]
	}
	// 其余位置填充随机值
	for i := len(testValues); i < plaintext.Slots(); i++ {
		values[i] = sampling.RandFloat64(-6, 6)
	}

	// 编码和加密
	if err := encoder.Encode(values, plaintext); err != nil {
		panic(err)
	}
	ciphertext, err := encryptor.EncryptNew(plaintext)
	if err != nil {
		panic(err)
	}

	// 测试不同的激活函数
	activations := []homomorphic.ActivationFunction{
		&homomorphic.IdentityActivation{},
		homomorphic.NewSigmoidChebyshevActivation(6.0, 20),
	}

	for _, activation := range activations {
		fmt.Printf("\n========================================\n")
		fmt.Printf("测试激活函数: %s\n", activation.GetName())
		fmt.Printf("========================================\n")

		// 测试导数计算
		testActivationDerivative(activation, ciphertext, values, testValues, evaluator, decryptor, encoder, params)
	}
}

func testActivationDerivative(
	activation homomorphic.ActivationFunction,
	ciphertext *rlwe.Ciphertext,
	values []float64,
	testValues []float64,
	evaluator *ckks.Evaluator,
	decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder,
	params ckks.Parameters,
) {
	fmt.Printf("输入密文层级: %d\n", ciphertext.Level())

	// 计算导数
	derivativeResult, err := activation.ApplyDerivative(ciphertext, evaluator, params)
	if err != nil {
		fmt.Printf("计算导数失败: %v\n", err)
		return
	}

	fmt.Printf("导数计算后密文层级: %d\n", derivativeResult.Level())

	// 解密结果
	ptResult := decryptor.DecryptNew(derivativeResult)
	encryptedDerivatives := make([]float64, len(values))
	if err := encoder.Decode(ptResult, encryptedDerivatives); err != nil {
		fmt.Printf("解密导数结果失败: %v\n", err)
		return
	}

	// 计算真实导数值用于对比
	trueDerivatives := make([]float64, len(values))
	for i, val := range values {
		trueDerivatives[i] = computeTrueDerivative(activation.GetName(), val)
	}

	// 打印对比结果
	fmt.Printf("\n%-10s %-15s %-15s %-15s %-15s\n",
		"输入", "真实导数", "加密导数", "绝对误差", "相对误差")
	fmt.Println("------------------------------------------------------------------------")

	maxError := 0.0
	avgError := 0.0
	validCount := 0

	for i := 0; i < len(testValues); i++ {
		if i >= len(values) {
			break
		}

		trueVal := trueDerivatives[i]
		encVal := encryptedDerivatives[i]
		absError := math.Abs(trueVal - encVal)

		var relError float64
		if math.Abs(trueVal) > 1e-10 {
			relError = absError / math.Abs(trueVal) * 100
		} else {
			relError = 0
		}

		fmt.Printf("%-10.2f %-15.10f %-15.10f %-15.2e %-15.2f%%\n",
			values[i], trueVal, encVal, absError, relError)

		if absError > maxError {
			maxError = absError
		}
		avgError += absError
		validCount++
	}

	if validCount > 0 {
		avgError /= float64(validCount)
	}

	fmt.Printf("\n误差统计：\n")
	fmt.Printf("最大绝对误差: %.2e\n", maxError)
	fmt.Printf("平均绝对误差: %.2e\n", avgError)

	// 导数特性分析
	fmt.Printf("\n导数特性分析:\n")
	switch activation.GetName() {
	case "Identity":
		fmt.Printf("- 恒等函数导数恒为1\n")
	case "SigmoidChebyshev(K=6.0, degree=20)":
		fmt.Printf("- Sigmoid导数: σ'(x) = σ(x) * (1 - σ(x))\n")
		fmt.Printf("- 最大值: σ'(0) = 0.25 (在x=0处)\n")
		fmt.Printf("- 对称性: 关于x=0对称\n")
		fmt.Printf("- 渐近行为: 当|x|→∞时，σ'(x)→0\n")

		// 验证关键点
		fmt.Printf("\n关键点验证:\n")
		x0_idx := -1
		for i := 0; i < len(testValues); i++ {
			if i < len(values) && math.Abs(values[i]) < 1e-10 { // 找到x=0的位置
				x0_idx = i
				break
			}
		}
		if x0_idx >= 0 && x0_idx < len(encryptedDerivatives) {
			fmt.Printf("x=0处: 真实值=%.6f, 近似值=%.6f, 误差=%.2e\n",
				trueDerivatives[x0_idx], encryptedDerivatives[x0_idx],
				math.Abs(encryptedDerivatives[x0_idx]-trueDerivatives[x0_idx]))
		}
	}
}

// computeTrueDerivative 计算真实的导数值
func computeTrueDerivative(activationName string, x float64) float64 {
	switch activationName {
	case "Identity":
		return 1.0
	case "SigmoidChebyshev(K=6.0, degree=20)":
		// Sigmoid导数: σ'(x) = σ(x) * (1 - σ(x))
		sigmoid := 1.0 / (1.0 + math.Exp(-x))
		return sigmoid * (1.0 - sigmoid)
	case "ExpChebyshev(K=10.0, degree=20)":
		// 指数函数导数就是自身
		return math.Exp(x)
	default:
		return 0.0
	}
}
