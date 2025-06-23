package main

import (
	"MPHEDev/pkg/dataProcess"
	"MPHEDev/pkg/network"
	"MPHEDev/pkg/training"

	"fmt"
	"log"
)

func main() {
	// 加载数据集
	trainDataset, testDataset, err := dataProcess.LoadDataset()
	if err != nil {
		log.Fatalf("加载数据集失败: %v", err)
	}

	// 打印数据集信息
	fmt.Printf("训练数据集包含 %d 个样本\n", len(trainDataset.Images))
	fmt.Printf("测试数据集包含 %d 个样本\n", len(testDataset.Images))

	// 定义神经网络
	inputSize := len(trainDataset.Images[0])
	numClasses := 10
	layerSizes := []int{inputSize, 64, 64, 64, numClasses} // 每一层的网络拥有的节点数

	// 创建神经网络
	nn := network.NewNeuronNetwork(layerSizes)

	// 创建差分隐私配置
	dpConfig := network.NewDPSGDConfig()
	dpConfig.L2NormClip = 1.0      // 梯度裁剪阈值，越小隐私保护越强
	dpConfig.NoiseMultiplier = 1.0 // 噪声乘数，越大隐私保护越强
	dpConfig.BatchSize = 128       // 批次大小
	dpConfig.LearningRate = 0.01   // 学习率
	dpConfig.Delta = 1e-5          // delta值

	// 训练轮数
	epochs := 20

	// 使用差分隐私训练模型
	fmt.Println("开始使用差分隐私SGD训练模型...")
	training.TrainModelWithDP(nn, trainDataset, testDataset, dpConfig, epochs, numClasses)

	// 展示一些测试样本的预测结果
	testInputs, _ := training.PrepareData(testDataset, numClasses)
	fmt.Println("\n测试样本预测结果:")
	for i := 0; i < 10; i++ {
		prediction := nn.Predict(testInputs[i])
		fmt.Printf("样本 %d 的预测类别：%d, 真实类别：%d\n", i+1, prediction, testDataset.Labels[i])
	}
}
