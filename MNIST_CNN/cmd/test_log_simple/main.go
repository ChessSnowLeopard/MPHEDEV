package main

import (
	"MNIST-CNN/pkg/homomorphic"
	"fmt"
	"math"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("简化的对数近似测试")
	fmt.Println("========================================")

	// 设置CKKS参数
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

	// 生成密钥
	kgen := rlwe.NewKeyGenerator(params)
	sk := kgen.GenSecretKeyNew()
	rlk := kgen.GenRelinearizationKeyNew(sk)
	evk := rlwe.NewMemEvaluationKeySet(rlk)

	// 创建编码器、加密器、解密器、评估器
	encoder := ckks.NewEncoder(params)
	encryptor := rlwe.NewEncryptor(params, sk)
	decryptor := rlwe.NewDecryptor(params, sk)
	evaluator := ckks.NewEvaluator(params, evk)

	// 创建对数近似器
	logApproximator := homomorphic.NewLogarithmChebyshevActivation(1.0, 0.01, 20)

	// 测试单个值
	testValues := []float64{0.1, 0.5, 0.8}

	fmt.Printf("测试对数近似器:\n")
	fmt.Printf("  区间: [%.3f, %.3f]\n", logApproximator.Eps, logApproximator.K)
	fmt.Printf("  阶数: %d\n", logApproximator.Degree)
	fmt.Printf("\n")

	slots := 1 << params.LogMaxSlots()
	fmt.Printf("CKKS槽数: %d\n", slots)

	for _, testVal := range testValues {
		fmt.Printf("\n--- 测试值: %.3f ---\n", testVal)

		// 准备输入向量
		values := make([]complex128, slots)
		values[0] = complex(testVal, 0)
		// 其他位置填充0
		for i := 1; i < slots; i++ {
			values[i] = complex(0, 0)
		}

		// 编码
		pt := ckks.NewPlaintext(params, params.MaxLevel())
		if err := encoder.Encode(values, pt); err != nil {
			fmt.Printf("编码失败: %v\n", err)
			continue
		}
		fmt.Printf("编码成功\n")

		// 加密
		ct, err := encryptor.EncryptNew(pt)
		if err != nil {
			fmt.Printf("加密失败: %v\n", err)
			continue
		}
		fmt.Printf("加密成功，层级: %d\n", ct.Level())

		// 应用对数近似
		logCt, err := logApproximator.Apply(ct, evaluator, params)
		if err != nil {
			fmt.Printf("对数近似失败: %v\n", err)
			continue
		}
		fmt.Printf("对数近似成功，层级: %d\n", logCt.Level())

		// 解密
		logPt := decryptor.DecryptNew(logCt)
		logResult := make([]complex128, slots)
		if err := encoder.Decode(logPt, logResult); err != nil {
			fmt.Printf("解码失败: %v\n", err)
			continue
		}
		fmt.Printf("解密解码成功\n")

		// 计算误差
		trueLog := math.Log(testVal)
		homomorphicLog := real(logResult[0])
		absError := math.Abs(trueLog - homomorphicLog)
		relError := absError / math.Abs(trueLog) * 100

		fmt.Printf("真实log(%.3f) = %.6f\n", testVal, trueLog)
		fmt.Printf("同态log(%.3f) = %.6f\n", testVal, homomorphicLog)
		fmt.Printf("绝对误差: %.6f\n", absError)
		fmt.Printf("相对误差: %.2f%%\n", relError)

		// 检查结果向量的前几个值
		fmt.Printf("结果向量前5个值: ")
		for i := 0; i < 5; i++ {
			fmt.Printf("%.6f ", real(logResult[i]))
		}
		fmt.Printf("\n")

		if absError < 1e-3 {
			fmt.Printf("✅ 误差可接受\n")
		} else {
			fmt.Printf("⚠️  误差较大\n")
		}
	}

	fmt.Println("\n========================================")
	fmt.Println("测试完成")
	fmt.Println("========================================")
}
