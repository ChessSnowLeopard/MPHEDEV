package test

import (
	"fmt"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"math"
	"math/rand"
)

// generateTestValues 生成测试用的复数值
func generateTestValues(slots int) []complex128 {
	r := rand.New(rand.NewSource(0))
	values := make([]complex128, slots)
	for i := 0; i < slots; i++ {
		values[i] = complex(2*r.Float64()-1, 2*r.Float64()-1)
	}
	return values
}

// complexAbs 计算复数的绝对值
func complexAbs(c complex128) float64 {
	return math.Sqrt(real(c)*real(c) + imag(c)*imag(c))
}

// PrintDebugInfo 打印调试信息
func PrintDebugInfo(params ckks.Parameters, ciphertext *rlwe.Ciphertext,
	valuesWant []complex128, decryptor *rlwe.Decryptor,
	encoder *ckks.Encoder) []complex128 {

	valuesTest := make([]complex128, ciphertext.Slots())

	if err := encoder.Decode(decryptor.DecryptNew(ciphertext), valuesTest); err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Printf("Level: %d (logQ = %d)\n", ciphertext.Level(), params.LogQLvl(ciphertext.Level()))
	fmt.Printf("Scale: 2^%f\n", math.Log2(ciphertext.Scale.Float64()))
	fmt.Printf("ValuesTest: %6.10f %6.10f %6.10f %6.10f...\n",
		valuesTest[0], valuesTest[1], valuesTest[2], valuesTest[3])
	fmt.Printf("ValuesWant: %6.10f %6.10f %6.10f %6.10f...\n",
		valuesWant[0], valuesWant[1], valuesWant[2], valuesWant[3])

	// 计算精度统计
	precStats := ckks.GetPrecisionStats(params, encoder, nil, valuesWant, valuesTest, 0, false)
	fmt.Println(precStats.String())
	fmt.Println()

	return valuesTest
}

// CompareResults 比较结果
func CompareResults(original, decrypted []complex128, name string) {
	if len(original) != len(decrypted) {
		fmt.Printf("%s: 长度不匹配\n", name)
		return
	}

	var sumErr float64
	for i := 0; i < len(original); i++ {
		diff := original[i] - decrypted[i]
		sumErr += complexAbs(diff) * complexAbs(diff)
	}
	avgErr := math.Sqrt(sumErr / float64(len(original)))

	fmt.Printf("%s 平均误差: %.10f\n", name, avgErr)

	if avgErr < 1e-10 {
		fmt.Printf("%s 测试通过\n", name)
	} else {
		fmt.Printf("%s 误差较大，需要检查\n", name)
	}
}
