package training

import (
	"MPHEDev/pkg/dataProcess"
	"MPHEDev/pkg/network"
	"fmt"
	"time"

	"gonum.org/v1/gonum/mat"
)

// OneHotEncode 将标签转换为one-hot编码
func OneHotEncode(label int, numClasses int) *mat.VecDense {
	oneHot := mat.NewVecDense(numClasses, nil)
	oneHot.SetVec(label, 1.0)
	return oneHot
}

// PrepareData 准备训练和测试数据
func PrepareData(dataset *dataProcess.Dataset, numClasses int) ([]*mat.VecDense, []*mat.VecDense) {
	inputs := make([]*mat.VecDense, len(dataset.Images))
	targets := make([]*mat.VecDense, len(dataset.Labels))

	inputSize := len(dataset.Images[0])

	for i := 0; i < len(dataset.Images); i++ {
		// 准备输入数据
		input := mat.NewVecDense(inputSize, nil)
		for j := 0; j < inputSize; j++ {
			// 归一化像素值到0-1之间
			input.SetVec(j, float64(dataset.Images[i][j])/255.0)
		}
		inputs[i] = input

		// 准备目标输出（one-hot编码）
		targets[i] = OneHotEncode(int(dataset.Labels[i]), numClasses)
	}

	return inputs, targets
}

// TrainModel 训练模型
func TrainModel(nn *network.NeuronNetwork, trainDataset *dataProcess.Dataset, testDataset *dataProcess.Dataset, batchSize int, learningRate float64, epochs int, numClasses int) {
	// 准备训练数据
	trainInputs, trainTargets := PrepareData(trainDataset, numClasses)

	// 准备测试数据
	testInputs, testTargets := PrepareData(testDataset, numClasses)

	// 训练前评估
	initialAccuracy := nn.Evaluate(testInputs, testTargets)
	initialLoss := nn.CalculateLoss(trainInputs, trainTargets)

	fmt.Printf("训练前 - 损失: %.4f, 准确率: %.2f%%\n", initialLoss, initialAccuracy*100)

	// 训练模型
	startTrain := time.Now()
	nn.Train(trainInputs, trainTargets, batchSize, learningRate, epochs)
	// 计算并打印训练时间
	elapsed := time.Since(startTrain)
	fmt.Printf("训练耗时: %v\n", elapsed) // 训练后评估
	startInterface := time.Now()
	finalAccuracy := nn.Evaluate(testInputs, testTargets)
	finalLoss := nn.CalculateLoss(trainInputs, trainTargets)
	elapsedInterface := time.Since(startInterface)
	fmt.Printf("推理耗时: %v\n", elapsedInterface)
	fmt.Printf("训练后 - 损失: %.4f, 准确率: %.2f%%\n", finalLoss, finalAccuracy*100)
}

// TrainModelWithDP 使用差分隐私训练模型
func TrainModelWithDP(nn *network.NeuronNetwork, trainDataset *dataProcess.Dataset, testDataset *dataProcess.Dataset, dpConfig *network.DPSGDConfig, epochs int, numClasses int) {
	// 准备训练数据
	trainInputs, trainTargets := PrepareData(trainDataset, numClasses)

	// 准备测试数据
	testInputs, testTargets := PrepareData(testDataset, numClasses)

	// 训练前评估
	initialAccuracy := nn.Evaluate(testInputs, testTargets)
	initialLoss := nn.CalculateLoss(trainInputs, trainTargets)

	fmt.Printf("训练前 - 损失: %.4f, 准确率: %.2f%%\n", initialLoss, initialAccuracy*100)
	fmt.Printf("差分隐私参数 - 噪声乘数: %.2f, 裁剪阈值: %.2f, 批次大小: %d, δ=%.0e\n",
		dpConfig.NoiseMultiplier, dpConfig.L2NormClip, dpConfig.BatchSize, dpConfig.Delta)

	// 使用差分隐私SGD训练模型
	startTrain := time.Now()
	lossHistory := nn.TrainWithDP(trainInputs, trainTargets, dpConfig, epochs)
	elapsed := time.Since(startTrain)
	fmt.Printf("训练耗时: %v\n", elapsed)

	// 训练后评估
	startInference := time.Now()
	finalAccuracy := nn.Evaluate(testInputs, testTargets)
	finalLoss := nn.CalculateLoss(trainInputs, trainTargets)
	elapsedInference := time.Since(startInference)

	fmt.Printf("推理耗时: %v\n", elapsedInference)
	fmt.Printf("训练后 - 损失: %.4f, 准确率: %.2f%%\n", finalLoss, finalAccuracy*100)

	// 打印最后几轮的准确率和损失
	lastEpochs := 5
	if epochs < lastEpochs {
		lastEpochs = epochs
	}
	fmt.Printf("\n最后 %d 轮训练结果:\n", lastEpochs)
	for i := epochs - lastEpochs; i < epochs; i++ {
		fmt.Printf("轮次 %d - 损失: %.4f,", i+1, lossHistory[i])
	}
}
