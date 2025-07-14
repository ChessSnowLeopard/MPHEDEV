package main

import (
	"MNIST-CNN/pkg/homomorphic"
	"fmt"
	"log"
	"math"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

func main() {
	fmt.Println("=== 指数和倒数函数精度测试 ===")

	// 初始化CKKS参数
	params, err := ckks.NewParametersFromLiteral(
		ckks.ParametersLiteral{
			LogN:            14,
			LogQ:            []int{55, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45},
			LogP:            []int{61},
			LogDefaultScale: 45,
		})
	if err != nil {
		log.Fatal(err)
	}

	// 生成密钥
	kgen := rlwe.NewKeyGenerator(params)
	sk := kgen.GenSecretKeyNew()
	rlk := kgen.GenRelinearizationKeyNew(sk)
	evk := rlwe.NewMemEvaluationKeySet(rlk)

	// 创建编码器、加密器、解密器和评估器
	encoder := ckks.NewEncoder(params)
	encryptor := rlwe.NewEncryptor(params, sk)
	decryptor := rlwe.NewDecryptor(params, sk)
	evaluator := ckks.NewEvaluator(params, evk)

	slots := 1 << params.LogMaxSlots()

	// 测试1: 指数函数
	fmt.Println("\n### 测试1: 指数函数 exp(x) ###")
	testExpFunction(params, encoder, encryptor, decryptor, evaluator, slots)

	// 测试2: 倒数函数
	fmt.Println("\n### 测试2: 倒数函数 1/x ###")
	testReciprocalFunction(params, encoder, encryptor, decryptor, evaluator, slots)

	// 测试3: 组合测试 - 模拟softmax中的计算
	fmt.Println("\n### 测试3: 组合测试 (模拟softmax) ###")
	testSoftmaxComponents(params, encoder, encryptor, decryptor, evaluator, slots)

	fmt.Println("\n=== 测试完成 ===")
}

func testExpFunction(params ckks.Parameters, encoder *ckks.Encoder, encryptor *rlwe.Encryptor,
	decryptor *rlwe.Decryptor, evaluator *ckks.Evaluator, slots int) {

	// 测试不同的指数函数参数
	testConfigs := []struct {
		K      float64
		Degree int
		Name   string
	}{
		{10.0, 20, "原始参数"},
		{5.0, 25, "调整后参数"},
		{3.0, 30, "更小区间"},
	}

	testValues := []float64{-2.0, -1.0, -0.5, 0.0, 0.5, 1.0, 2.0, 2.5}

	for _, config := range testConfigs {
		fmt.Printf("\n测试配置: %s (K=%.1f, Degree=%d)\n", config.Name, config.K, config.Degree)

		// 创建激活函数
		expActivation := homomorphic.NewExpChebyshevActivation(config.K, config.Degree)

		// 准备数据
		values := make([]complex128, slots)
		for i := 0; i < len(testValues) && i < slots; i++ {
			values[i] = complex(testValues[i], 0)
		}

		// 加密
		pt := ckks.NewPlaintext(params, params.MaxLevel())
		encoder.Encode(values, pt)
		ct, _ := encryptor.EncryptNew(pt)

		// 应用指数函数
		result, err := expActivation.Apply(ct, evaluator, params)
		if err != nil {
			fmt.Printf("计算失败: %v\n", err)
			continue
		}

		// 解密
		ptResult := decryptor.DecryptNew(result)
		decoded := make([]complex128, slots)
		encoder.Decode(ptResult, decoded)

		// 比较结果
		fmt.Println("x     | exp(x)真值 | 同态计算 | 绝对误差 | 相对误差")
		fmt.Println("------|------------|----------|----------|----------")

		maxAbsError := 0.0
		maxRelError := 0.0

		for i := 0; i < len(testValues); i++ {
			x := testValues[i]
			trueValue := math.Exp(x)
			computedValue := real(decoded[i])
			absError := math.Abs(trueValue - computedValue)
			relError := absError / trueValue

			maxAbsError = math.Max(maxAbsError, absError)
			maxRelError = math.Max(maxRelError, relError)

			fmt.Printf("%5.1f | %10.6f | %8.6f | %8.3e | %8.1f%%\n",
				x, trueValue, computedValue, absError, relError*100)
		}

		fmt.Printf("最大绝对误差: %.3e, 最大相对误差: %.1f%%\n", maxAbsError, maxRelError*100)
	}
}

func testReciprocalFunction(params ckks.Parameters, encoder *ckks.Encoder, encryptor *rlwe.Encryptor,
	decryptor *rlwe.Decryptor, evaluator *ckks.Evaluator, slots int) {

	// 测试不同的倒数函数参数
	testConfigs := []struct {
		K       float64
		Epsilon float64
		Degree  int
		Name    string
	}{
		{100.0, 0.1, 20, "原始参数"},
		{50.0, 0.5, 25, "调整后参数"},
		{20.0, 1.0, 30, "更保守参数"},
	}

	// 测试exp(x)的和，模拟softmax中的情况
	testValues := []float64{7.0, 8.0, 9.0, 10.0, 15.0, 20.0}

	for _, config := range testConfigs {
		fmt.Printf("\n测试配置: %s (K=%.1f, Eps=%.1f, Degree=%d)\n",
			config.Name, config.K, config.Epsilon, config.Degree)

		// 创建激活函数
		recipActivation := homomorphic.NewReciprocalChebyshevActivation(config.K, config.Epsilon, config.Degree)

		// 准备数据
		values := make([]complex128, slots)
		for i := 0; i < len(testValues) && i < slots; i++ {
			values[i] = complex(testValues[i], 0)
		}

		// 加密
		pt := ckks.NewPlaintext(params, params.MaxLevel())
		encoder.Encode(values, pt)
		ct, _ := encryptor.EncryptNew(pt)

		// 应用倒数函数
		result, err := recipActivation.Apply(ct, evaluator, params)
		if err != nil {
			fmt.Printf("计算失败: %v\n", err)
			continue
		}

		// 解密
		ptResult := decryptor.DecryptNew(result)
		decoded := make([]complex128, slots)
		encoder.Decode(ptResult, decoded)

		// 比较结果
		fmt.Println("x     | 1/x真值    | 同态计算 | 绝对误差 | 相对误差")
		fmt.Println("------|------------|----------|----------|----------")

		maxAbsError := 0.0
		maxRelError := 0.0

		for i := 0; i < len(testValues); i++ {
			x := testValues[i]
			trueValue := 1.0 / x
			computedValue := real(decoded[i])
			absError := math.Abs(trueValue - computedValue)
			relError := absError / trueValue

			maxAbsError = math.Max(maxAbsError, absError)
			maxRelError = math.Max(maxRelError, relError)

			fmt.Printf("%5.1f | %10.6f | %8.6f | %8.3e | %8.1f%%\n",
				x, trueValue, computedValue, absError, relError*100)
		}

		fmt.Printf("最大绝对误差: %.3e, 最大相对误差: %.1f%%\n", maxAbsError, maxRelError*100)
	}
}

func testSoftmaxComponents(params ckks.Parameters, encoder *ckks.Encoder, encryptor *rlwe.Encryptor,
	decryptor *rlwe.Decryptor, evaluator *ckks.Evaluator, slots int) {

	// 模拟神经网络输出
	outputs := []float64{2.0, -0.5, -0.4, -0.3}

	fmt.Println("\n原始输出:", outputs)

	// 步骤1: 计算exp
	fmt.Println("\n步骤1: 计算exp(x)")
	expValues := make([]float64, len(outputs))
	expSum := 0.0

	for i, x := range outputs {
		expValues[i] = math.Exp(x)
		expSum += expValues[i]
		fmt.Printf("exp(%.1f) = %.6f\n", x, expValues[i])
	}
	fmt.Printf("sum(exp) = %.6f\n", expSum)

	// 步骤2: 计算1/sum
	fmt.Printf("\n步骤2: 计算1/sum = 1/%.6f = %.6f\n", expSum, 1.0/expSum)

	// 步骤3: 计算softmax
	fmt.Println("\n步骤3: 计算softmax概率")
	for i, expVal := range expValues {
		prob := expVal / expSum
		fmt.Printf("softmax[%d] = %.6f / %.6f = %.6f\n", i, expVal, expSum, prob)
	}

	// 现在用同态加密计算
	fmt.Println("\n使用同态加密计算...")

	// 使用调整后的参数
	expActivation := homomorphic.NewExpChebyshevActivation(5.0, 25)
	recipActivation := homomorphic.NewReciprocalChebyshevActivation(50.0, 0.5, 25)

	// 加密输出并计算exp
	expResults := make([]*rlwe.Ciphertext, len(outputs))
	for i, x := range outputs {
		values := make([]complex128, slots)
		values[0] = complex(x, 0)

		pt := ckks.NewPlaintext(params, params.MaxLevel())
		encoder.Encode(values, pt)
		ct, _ := encryptor.EncryptNew(pt)

		expResults[i], _ = expActivation.Apply(ct, evaluator, params)
	}

	// 计算sum
	sumCt := expResults[0].CopyNew()
	for i := 1; i < len(expResults); i++ {
		sumCt, _ = evaluator.AddNew(sumCt, expResults[i])
	}

	// 计算1/sum
	recipSumCt, _ := recipActivation.Apply(sumCt, evaluator, params)

	// 计算softmax
	fmt.Println("\n同态计算结果:")
	for i := 0; i < len(outputs); i++ {
		// softmax[i] = exp[i] * (1/sum)
		softmaxCt, _ := evaluator.MulNew(expResults[i], recipSumCt)
		evaluator.Rescale(softmaxCt, softmaxCt)

		// 解密
		pt := decryptor.DecryptNew(softmaxCt)
		decoded := make([]complex128, slots)
		encoder.Decode(pt, decoded)

		trueProb := expValues[i] / expSum
		computedProb := real(decoded[0])
		error := math.Abs(trueProb - computedProb)

		fmt.Printf("softmax[%d]: 真值=%.6f, 计算=%.6f, 误差=%.3e\n",
			i, trueProb, computedProb, error)
	}
}
