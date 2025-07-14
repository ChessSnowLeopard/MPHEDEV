package main

import (
	"MNIST-CNN/pkg/homomorphic"
	"MNIST-CNN/pkg/setup"
	"MNIST-CNN/pkg/training"
	"MNIST-CNN/pkg/vector_processor"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

func main() {
	fmt.Println("=== MNIST Softmax 多层网络测试 ===")

	// 初始化系统
	params, err := setup.InitParameters()
	if err != nil {
		log.Fatalf("参数初始化失败: %v", err)
	}

	// 简单密钥生成（单方测试）
	kgen := rlwe.NewKeyGenerator(params)
	sk := kgen.GenSecretKeyNew()
	pk := kgen.GenPublicKeyNew(sk)
	rlk := kgen.GenRelinearizationKeyNew(sk)

	// 生成必要的伽罗瓦密钥
	galEls := []uint64{
		params.GaloisElement(1),
		params.GaloisElement(64),
		params.GaloisElement(128),
		params.GaloisElement(256),
	}
	gks := make([]*rlwe.GaloisKey, len(galEls))
	for i, galEl := range galEls {
		gks[i] = kgen.GenGaloisKeyNew(galEl, sk)
	}

	// 创建编码器等
	slots := 1 << params.LogMaxSlots()
	encoder := ckks.NewEncoder(params)
	encryptor := ckks.NewEncryptor(params, pk)
	decryptor := ckks.NewDecryptor(params, sk)

	// 创建评估器
	evk := rlwe.NewMemEvaluationKeySet(rlk, gks...)
	evaluator := ckks.NewEvaluator(params, evk)

	// 加载数据集
	fmt.Println("\n加载MNIST数据集...")
	trainDataset, _, err := training.LoadDataset()
	if err != nil {
		log.Fatalf("加载数据集失败: %v", err)
	}

	// 配置向量组装器
	assembler := vector_processor.NewVectorAssemblerFromDataset(slots, 64, trainDataset)

	// 处理一个批次
	numImages := assembler.ImagesPerVector
	smallDataset := &training.Dataset{
		Images: trainDataset.Images[:numImages],
		Labels: trainDataset.Labels[:numImages],
	}

	vectors, batchLabels, err := assembler.AssembleDataset(smallDataset)
	if err != nil {
		log.Fatalf("数据重排失败: %v", err)
	}

	// 加密数据
	fmt.Println("\n加密数据...")
	encryptedVectors := make(map[uint64]*rlwe.Ciphertext)
	for idx, vector := range vectors {
		complexVector := make([]complex128, slots)
		for i := 0; i < len(vector) && i < slots; i++ {
			complexVector[i] = complex(float64(vector[i]), 0)
		}

		pt := ckks.NewPlaintext(params, params.MaxLevel())
		encoder.Encode(complexVector, pt)
		ct, _ := encryptor.EncryptNew(pt)
		encryptedVectors[uint64(idx)] = ct
	}

	// 创建一个3层网络：784 -> 16 -> 10
	fmt.Println("\n创建神经网络...")
	layerSizes := []int{784, 16, 10}
	activations := []string{"sigmoid_chebyshev", "sigmoid_chebyshev"}
	mlp := homomorphic.NewMLPWithActivations(layerSizes, activations, assembler)
	mlp.PrintArchitecture()

	// 执行前向传播
	fmt.Println("\n执行前向传播...")
	startTime := time.Now()
	outputs, err := mlp.ForwardPropagation(encryptedVectors, 0, params, evaluator, encoder, slots)
	if err != nil {
		log.Fatalf("前向传播失败: %v", err)
	}
	forwardTime := time.Since(startTime)
	fmt.Printf("前向传播完成，用时: %v\n", forwardTime)

	// 应用Softmax
	fmt.Println("\n应用Softmax激活函数...")
	startTime = time.Now()
	softmaxOutputs, err := homomorphic.HomomorphicSoftmax(outputs, params, evaluator, encoder, slots)
	if err != nil {
		log.Fatalf("Softmax计算失败: %v", err)
	}
	softmaxTime := time.Since(startTime)
	fmt.Printf("Softmax计算完成，用时: %v\n", softmaxTime)

	// 验证结果
	homomorphic.VerifySoftmax(outputs, softmaxOutputs, decryptor, encoder, slots, assembler.ImagesPerVector)

	// 分析预测准确率
	fmt.Println("\n=== 预测准确率分析 ===")
	comparePredictions(outputs, softmaxOutputs, batchLabels[0], decryptor, encoder, slots)

	fmt.Printf("\n=== 测试完成 ===\n")
	fmt.Printf("总用时: %v\n", forwardTime+softmaxTime)
}

func comparePredictions(rawOutputs []*rlwe.Ciphertext, softmaxOutputs []*rlwe.Ciphertext,
	labels []byte, decryptor *rlwe.Decryptor, encoder *ckks.Encoder, slots int) {

	n := len(labels)

	// 解密并找到预测类别
	rawPredictions := make([]int, n)
	softmaxPredictions := make([]int, n)
	confidences := make([]float64, n) // 存储softmax的置信度

	for sampleIdx := 0; sampleIdx < n; sampleIdx++ {
		maxRaw := -math.MaxFloat64
		maxSoftmax := -math.MaxFloat64
		rawIdx := -1
		softmaxIdx := -1

		for neuronIdx, ct := range rawOutputs {
			// 原始输出
			pt := decryptor.DecryptNew(ct)
			decoded := make([]complex128, slots)
			encoder.Decode(pt, decoded)
			val := real(decoded[sampleIdx])
			if val > maxRaw {
				maxRaw = val
				rawIdx = neuronIdx
			}

			// Softmax输出
			pt = decryptor.DecryptNew(softmaxOutputs[neuronIdx])
			encoder.Decode(pt, decoded)
			val = real(decoded[sampleIdx])
			if val > maxSoftmax {
				maxSoftmax = val
				softmaxIdx = neuronIdx
			}
		}

		rawPredictions[sampleIdx] = rawIdx
		softmaxPredictions[sampleIdx] = softmaxIdx
		confidences[sampleIdx] = maxSoftmax
	}

	// 计算准确率
	rawCorrect := 0
	softmaxCorrect := 0

	fmt.Println("\n预测结果（前10个样本）：")
	fmt.Println("样本 | 真实标签 | 原始预测 | Softmax预测 | 置信度  | 原始正确 | Softmax正确")
	fmt.Println("-----|----------|----------|-------------|---------|----------|-------------")

	for i := 0; i < min(10, n); i++ {
		trueLabel := int(labels[i])
		rawPred := rawPredictions[i]
		softPred := softmaxPredictions[i]
		confidence := confidences[i]

		rawOK := rawPred == trueLabel
		softOK := softPred == trueLabel

		if rawOK {
			rawCorrect++
		}
		if softOK {
			softmaxCorrect++
		}

		fmt.Printf("%4d | %8d | %8d | %11d | %6.2f%% | %8s | %11s\n",
			i, trueLabel, rawPred, softPred, confidence*100,
			func() string {
				if rawOK {
					return "✓"
				}
				return "✗"
			}(),
			func() string {
				if softOK {
					return "✓"
				}
				return "✗"
			}())
	}

	// 计算全部样本的准确率
	for i := 10; i < n; i++ {
		if rawPredictions[i] == int(labels[i]) {
			rawCorrect++
		}
		if softmaxPredictions[i] == int(labels[i]) {
			softmaxCorrect++
		}
	}

	fmt.Printf("\n准确率统计（共%d个样本）：\n", n)
	fmt.Printf("原始输出准确率: %d/%d = %.2f%%\n", rawCorrect, n, float64(rawCorrect)/float64(n)*100)
	fmt.Printf("Softmax准确率: %d/%d = %.2f%%\n", softmaxCorrect, n, float64(softmaxCorrect)/float64(n)*100)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
