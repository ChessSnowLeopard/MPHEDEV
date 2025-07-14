package homomorphic

import (
	"fmt"
	"math"
)

// LRSchedulerType 学习率调度器类型
type LRSchedulerType int

const (
	StepLR            LRSchedulerType = iota // 步进衰减
	ExponentialLR                            // 指数衰减
	CosineAnnealingLR                        // 余弦退火
	ReduceLROnPlateau                        // 平台自适应衰减
	LinearLR                                 // 线性衰减
	WarmupLR                                 // 预热学习率
)

// LRScheduler 学习率调度器
type LRScheduler struct {
	InitialLR     float64         // 初始学习率
	CurrentLR     float64         // 当前学习率
	SchedulerType LRSchedulerType // 调度器类型

	// 步进衰减参数
	StepSize int     // 步长
	Gamma    float64 // 衰减因子

	// 指数衰减参数
	ExponentialGamma float64 // 指数衰减因子

	// 余弦退火参数
	T_max  int     // 最大epoch数
	EtaMin float64 // 最小学习率

	// 平台自适应衰减参数
	BestLoss     float64 // 最佳损失
	Patience     int     // 耐心值
	WaitCount    int     // 等待计数
	ReduceFactor float64 // 衰减因子
	MinLR        float64 // 最小学习率

	// 线性衰减参数
	TotalSteps int // 总步数

	// 预热参数
	WarmupSteps int // 预热步数

	// 当前步数/epoch
	CurrentStep  int
	CurrentEpoch int
}

// NewStepLRScheduler 创建步进衰减调度器
func NewStepLRScheduler(initialLR float64, stepSize int, gamma float64) *LRScheduler {
	return &LRScheduler{
		InitialLR:     initialLR,
		CurrentLR:     initialLR,
		SchedulerType: StepLR,
		StepSize:      stepSize,
		Gamma:         gamma,
	}
}

// NewExponentialLRScheduler 创建指数衰减调度器
func NewExponentialLRScheduler(initialLR float64, gamma float64) *LRScheduler {
	return &LRScheduler{
		InitialLR:        initialLR,
		CurrentLR:        initialLR,
		SchedulerType:    ExponentialLR,
		ExponentialGamma: gamma,
	}
}

// NewCosineAnnealingLRScheduler 创建余弦退火调度器
func NewCosineAnnealingLRScheduler(initialLR float64, T_max int, etaMin float64) *LRScheduler {
	return &LRScheduler{
		InitialLR:     initialLR,
		CurrentLR:     initialLR,
		SchedulerType: CosineAnnealingLR,
		T_max:         T_max,
		EtaMin:        etaMin,
	}
}

// NewReduceLROnPlateauScheduler 创建平台自适应衰减调度器
func NewReduceLROnPlateauScheduler(initialLR float64, patience int, reduceFactor float64, minLR float64) *LRScheduler {
	return &LRScheduler{
		InitialLR:     initialLR,
		CurrentLR:     initialLR,
		SchedulerType: ReduceLROnPlateau,
		BestLoss:      math.Inf(1), // 初始化为正无穷
		Patience:      patience,
		ReduceFactor:  reduceFactor,
		MinLR:         minLR,
	}
}

// NewLinearLRScheduler 创建线性衰减调度器
func NewLinearLRScheduler(initialLR float64, totalSteps int) *LRScheduler {
	return &LRScheduler{
		InitialLR:     initialLR,
		CurrentLR:     initialLR,
		SchedulerType: LinearLR,
		TotalSteps:    totalSteps,
	}
}

// NewWarmupLRScheduler 创建预热学习率调度器
func NewWarmupLRScheduler(initialLR float64, warmupSteps int) *LRScheduler {
	return &LRScheduler{
		InitialLR:     initialLR,
		CurrentLR:     0.0, // 从0开始预热
		SchedulerType: WarmupLR,
		WarmupSteps:   warmupSteps,
	}
}

// Step 更新学习率（基于epoch）
func (lr *LRScheduler) Step() {
	lr.CurrentEpoch++
	lr.updateLearningRate()
}

// StepWithLoss 更新学习率（基于损失，用于平台自适应衰减）
func (lr *LRScheduler) StepWithLoss(loss float64) {
	if lr.SchedulerType == ReduceLROnPlateau {
		if loss < lr.BestLoss {
			lr.BestLoss = loss
			lr.WaitCount = 0
		} else {
			lr.WaitCount++
			if lr.WaitCount >= lr.Patience {
				lr.CurrentLR = math.Max(lr.CurrentLR*lr.ReduceFactor, lr.MinLR)
				lr.WaitCount = 0
				fmt.Printf("🔻 学习率衰减: %.6f\n", lr.CurrentLR)
			}
		}
	} else {
		lr.Step()
	}
}

// StepBatch 更新学习率（基于batch步数）
func (lr *LRScheduler) StepBatch() {
	lr.CurrentStep++
	lr.updateLearningRate()
}

// updateLearningRate 根据调度器类型更新学习率
func (lr *LRScheduler) updateLearningRate() {
	switch lr.SchedulerType {
	case StepLR:
		lr.updateStepLR()
	case ExponentialLR:
		lr.updateExponentialLR()
	case CosineAnnealingLR:
		lr.updateCosineAnnealingLR()
	case LinearLR:
		lr.updateLinearLR()
	case WarmupLR:
		lr.updateWarmupLR()
	}
}

// updateStepLR 步进衰减
func (lr *LRScheduler) updateStepLR() {
	if lr.CurrentEpoch > 0 && lr.CurrentEpoch%lr.StepSize == 0 {
		lr.CurrentLR = lr.CurrentLR * lr.Gamma
		fmt.Printf("🔻 步进衰减: Epoch %d, 学习率 %.6f\n", lr.CurrentEpoch, lr.CurrentLR)
	}
}

// updateExponentialLR 指数衰减
func (lr *LRScheduler) updateExponentialLR() {
	lr.CurrentLR = lr.InitialLR * math.Pow(lr.ExponentialGamma, float64(lr.CurrentEpoch))
	if lr.CurrentEpoch%5 == 0 && lr.CurrentEpoch > 0 {
		fmt.Printf("📉 指数衰减: Epoch %d, 学习率 %.6f\n", lr.CurrentEpoch, lr.CurrentLR)
	}
}

// updateCosineAnnealingLR 余弦退火
func (lr *LRScheduler) updateCosineAnnealingLR() {
	lr.CurrentLR = lr.EtaMin + (lr.InitialLR-lr.EtaMin)*
		(1+math.Cos(math.Pi*float64(lr.CurrentEpoch)/float64(lr.T_max)))/2
	if lr.CurrentEpoch%5 == 0 && lr.CurrentEpoch > 0 {
		fmt.Printf("🌊 余弦退火: Epoch %d, 学习率 %.6f\n", lr.CurrentEpoch, lr.CurrentLR)
	}
}

// updateLinearLR 线性衰减
func (lr *LRScheduler) updateLinearLR() {
	factor := math.Max(0.0, 1.0-float64(lr.CurrentStep)/float64(lr.TotalSteps))
	lr.CurrentLR = lr.InitialLR * factor
}

// updateWarmupLR 预热学习率
func (lr *LRScheduler) updateWarmupLR() {
	if lr.CurrentStep < lr.WarmupSteps {
		lr.CurrentLR = lr.InitialLR * float64(lr.CurrentStep) / float64(lr.WarmupSteps)
	} else {
		lr.CurrentLR = lr.InitialLR
	}
}

// GetLR 获取当前学习率
func (lr *LRScheduler) GetLR() float64 {
	return lr.CurrentLR
}

// Reset 重置调度器
func (lr *LRScheduler) Reset() {
	lr.CurrentLR = lr.InitialLR
	lr.CurrentStep = 0
	lr.CurrentEpoch = 0
	lr.WaitCount = 0
	lr.BestLoss = math.Inf(1)
}

// GetSchedulerInfo 获取调度器信息
func (lr *LRScheduler) GetSchedulerInfo() string {
	switch lr.SchedulerType {
	case StepLR:
		return fmt.Sprintf("步进衰减(步长:%d, γ:%.3f)", lr.StepSize, lr.Gamma)
	case ExponentialLR:
		return fmt.Sprintf("指数衰减(γ:%.3f)", lr.ExponentialGamma)
	case CosineAnnealingLR:
		return fmt.Sprintf("余弦退火(T_max:%d, η_min:%.6f)", lr.T_max, lr.EtaMin)
	case ReduceLROnPlateau:
		return fmt.Sprintf("平台自适应(耐心:%d, 因子:%.3f)", lr.Patience, lr.ReduceFactor)
	case LinearLR:
		return fmt.Sprintf("线性衰减(总步数:%d)", lr.TotalSteps)
	case WarmupLR:
		return fmt.Sprintf("预热学习率(预热步数:%d)", lr.WarmupSteps)
	default:
		return "未知调度器"
	}
}

// PrintSchedule 打印学习率调度计划（用于预览）
func (lr *LRScheduler) PrintSchedule(epochs int) {
	fmt.Printf("\n=== 📈 学习率调度计划预览 ===\n")
	fmt.Printf("调度器类型: %s\n", lr.GetSchedulerInfo())
	fmt.Printf("初始学习率: %.6f\n\n", lr.InitialLR)

	// 备份当前状态
	originalEpoch := lr.CurrentEpoch
	originalStep := lr.CurrentStep
	originalLR := lr.CurrentLR
	originalWaitCount := lr.WaitCount
	originalBestLoss := lr.BestLoss

	// 重置并预览
	lr.Reset()

	fmt.Printf("Epoch | 学习率\n")
	fmt.Printf("------|----------\n")
	for i := 0; i <= epochs && i <= 20; i++ { // 最多显示20个epoch
		if i > 0 {
			lr.Step()
		}
		fmt.Printf("%5d | %.6f\n", i, lr.CurrentLR)
	}

	if epochs > 20 {
		fmt.Printf("  ... | ...\n")
		// 跳到最后几个epoch
		lr.CurrentEpoch = epochs - 2
		lr.updateLearningRate()
		fmt.Printf("%5d | %.6f\n", epochs-1, lr.CurrentLR)
		lr.Step()
		fmt.Printf("%5d | %.6f\n", epochs, lr.CurrentLR)
	}

	// 恢复原状态
	lr.CurrentEpoch = originalEpoch
	lr.CurrentStep = originalStep
	lr.CurrentLR = originalLR
	lr.WaitCount = originalWaitCount
	lr.BestLoss = originalBestLoss

	fmt.Printf("==============================\n")
}
