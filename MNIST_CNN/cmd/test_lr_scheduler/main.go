package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"MNIST-CNN/pkg/homomorphic"
)

func main() {
	fmt.Println("🚀 学习率调度器功能演示")
	fmt.Println(strings.Repeat("=", 50))

	// 1. 步进衰减演示
	fmt.Println("\n1️⃣ 步进衰减调度器演示")
	stepScheduler := homomorphic.NewStepLRScheduler(0.01, 5, 0.5)
	stepScheduler.PrintSchedule(20)

	// 2. 指数衰减演示
	fmt.Println("\n2️⃣ 指数衰减调度器演示")
	expScheduler := homomorphic.NewExponentialLRScheduler(0.01, 0.95)
	expScheduler.PrintSchedule(20)

	// 3. 余弦退火演示
	fmt.Println("\n3️⃣ 余弦退火调度器演示")
	cosineScheduler := homomorphic.NewCosineAnnealingLRScheduler(0.01, 20, 0.001)
	cosineScheduler.PrintSchedule(20)

	// 4. 平台自适应衰减演示
	fmt.Println("\n4️⃣ 平台自适应衰减调度器演示")
	plateauScheduler := homomorphic.NewReduceLROnPlateauScheduler(0.01, 3, 0.5, 0.0001)
	simulatePlateuScheduler(plateauScheduler)

	// 5. 线性衰减演示
	fmt.Println("\n5️⃣ 线性衰减调度器演示")
	linearScheduler := homomorphic.NewLinearLRScheduler(0.01, 100)
	demonstrateLinearScheduler(linearScheduler)

	// 6. 预热学习率演示
	fmt.Println("\n6️⃣ 预热学习率调度器演示")
	warmupScheduler := homomorphic.NewWarmupLRScheduler(0.01, 10)
	demonstrateWarmupScheduler(warmupScheduler)

	// 7. 实际训练循环集成演示
	fmt.Println("\n7️⃣ 实际训练循环集成演示")
	demonstrateTrainingIntegration()
}

// 模拟平台自适应衰减
func simulatePlateuScheduler(scheduler *homomorphic.LRScheduler) {
	fmt.Printf("调度器类型: %s\n", scheduler.GetSchedulerInfo())
	fmt.Printf("初始学习率: %.6f\n\n", scheduler.GetLR())

	// 模拟损失变化
	losses := []float64{
		2.5, 2.3, 2.1, 1.9, 1.8, 1.8, 1.8, 1.8, // 损失停止下降
		1.8, 1.8, 1.7, 1.6, 1.5, 1.5, 1.5, 1.5, // 又停止下降
		1.4, 1.3, 1.2, 1.1,
	}

	fmt.Printf("Epoch | 损失   | 学习率  | 状态\n")
	fmt.Printf("------|--------|---------|--------\n")

	for epoch, loss := range losses {
		oldLR := scheduler.GetLR()
		scheduler.StepWithLoss(loss)
		newLR := scheduler.GetLR()

		status := "正常"
		if newLR < oldLR {
			status = "📉衰减"
		}

		fmt.Printf("%5d | %.4f | %.6f | %s\n", epoch+1, loss, newLR, status)
	}
	fmt.Printf("==============================\n")
}

// 演示线性衰减
func demonstrateLinearScheduler(scheduler *homomorphic.LRScheduler) {
	fmt.Printf("调度器类型: %s\n", scheduler.GetSchedulerInfo())
	fmt.Printf("初始学习率: %.6f\n\n", scheduler.GetLR())

	fmt.Printf("步数  | 学习率\n")
	fmt.Printf("------|----------\n")

	for step := 0; step <= 100; step += 10 {
		for i := 0; i < 10 && scheduler.CurrentStep < 100; i++ {
			scheduler.StepBatch()
		}
		fmt.Printf("%5d | %.6f\n", step, scheduler.GetLR())
	}
	fmt.Printf("==============================\n")
}

// 演示预热学习率
func demonstrateWarmupScheduler(scheduler *homomorphic.LRScheduler) {
	fmt.Printf("调度器类型: %s\n", scheduler.GetSchedulerInfo())
	fmt.Printf("目标学习率: %.6f\n\n", scheduler.InitialLR)

	fmt.Printf("步数  | 学习率\n")
	fmt.Printf("------|----------\n")

	for step := 0; step <= 15; step++ {
		fmt.Printf("%5d | %.6f\n", step, scheduler.GetLR())
		if step < 15 {
			scheduler.StepBatch()
		}
	}
	fmt.Printf("==============================\n")
}

// 演示训练循环集成
func demonstrateTrainingIntegration() {
	fmt.Println("这里演示如何在实际训练中集成学习率调度器")

	// 创建调度器
	scheduler := homomorphic.NewStepLRScheduler(0.01, 3, 0.8)

	fmt.Printf("\n📚 模拟训练过程:\n")
	fmt.Printf("调度器: %s\n", scheduler.GetSchedulerInfo())
	fmt.Printf("初始学习率: %.6f\n\n", scheduler.GetLR())

	// 模拟多个epoch的训练
	for epoch := 0; epoch < 10; epoch++ {
		currentLR := scheduler.GetLR()

		// 模拟一个epoch的训练
		simulatedLoss := simulateEpochTraining(currentLR, epoch)

		fmt.Printf("Epoch %2d | 学习率: %.6f | 损失: %.4f\n",
			epoch+1, currentLR, simulatedLoss)

		// 在epoch结束时更新学习率
		scheduler.Step()
	}

	fmt.Println("\n✅ 训练完成！")
	fmt.Printf("最终学习率: %.6f\n", scheduler.GetLR())
}

// 模拟一个epoch的训练
func simulateEpochTraining(learningRate float64, epoch int) float64 {
	// 模拟训练过程，这里只是简单的随机生成
	rand.Seed(time.Now().UnixNano())
	baseLoss := 2.0 - float64(epoch)*0.1
	noise := (rand.Float64() - 0.5) * 0.2
	return baseLoss + noise
}

// 展示如何在实际训练中使用
func showRealWorldUsage() {
	fmt.Println("\n🔧 实际使用方法:")
	fmt.Println(`
// 1. 创建学习率调度器
scheduler := homomorphic.NewStepLRScheduler(0.01, 5, 0.5)

// 2. 在训练循环中使用
for epoch := 0; epoch < totalEpochs; epoch++ {
    // 获取当前学习率
    currentLR := scheduler.GetLR()
    
    // 更新神经网络的学习率
    mlp.SetLearningRate(currentLR)
    
    // 执行一个epoch的训练
    // ... 前向传播、损失计算、梯度计算、参数更新 ...
    
    // 在epoch结束时更新学习率
    scheduler.Step()
    
    // 对于平台自适应衰减，使用损失更新
    // scheduler.StepWithLoss(validationLoss)
}
`)
}
