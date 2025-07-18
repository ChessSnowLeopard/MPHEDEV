package main

import (
	"MPHEDev/pkg/training"
	"fmt"
	"log"
)

func main() {
	fmt.Println("测试数据集加载...")

	// 尝试加载数据集
	trainDataset, testDataset, err := training.LoadDataset()
	if err != nil {
		log.Fatalf("加载数据集失败: %v", err)
	}

	fmt.Printf("✓ 训练数据集加载成功: %d 个样本\n", len(trainDataset.Images))
	fmt.Printf("✓ 测试数据集加载成功: %d 个样本\n", len(testDataset.Images))
	fmt.Printf("✓ 训练标签数量: %d\n", len(trainDataset.Labels))
	fmt.Printf("✓ 测试标签数量: %d\n", len(testDataset.Labels))

	fmt.Println("数据集加载测试完成！")
}
