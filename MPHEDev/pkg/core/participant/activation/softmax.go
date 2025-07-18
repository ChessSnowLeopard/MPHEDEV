package activation

import (
	"MPHEDev/pkg/core/participant/interfaces"
	"fmt"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// SoftmaxProcessor Softmax处理器实现
type SoftmaxProcessor struct {
	expActivation        interfaces.ActivationFunction
	reciprocalActivation interfaces.ActivationFunction
}

// NewSoftmaxProcessor 创建Softmax处理器
func NewSoftmaxProcessor() *SoftmaxProcessor {
	return &SoftmaxProcessor{
		expActivation:        GetActivation("exp_chebyshev"),
		reciprocalActivation: GetActivation("reciprocal_chebyshev"),
	}
}

// NewSoftmaxProcessorWithParams 创建带自定义参数的Softmax处理器
func NewSoftmaxProcessorWithParams(expK, expDegree float64, recipK, recipEps, recipDegree float64) *SoftmaxProcessor {
	return &SoftmaxProcessor{
		expActivation:        NewExpChebyshevActivation(expK, int(expDegree)),
		reciprocalActivation: NewReciprocalChebyshevActivation(recipK, recipEps, int(recipDegree)),
	}
}

// ApplySoftmax 应用Softmax函数
func (sp *SoftmaxProcessor) ApplySoftmax(
	neuronOutputs []*rlwe.Ciphertext,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder) ([]*rlwe.Ciphertext, error) {

	fmt.Println("\n=== 计算同态Softmax ===")

	// 步骤1：对每个输出计算指数
	fmt.Printf("\n步骤1：计算exp(O_i)，共%d个神经元\n", len(neuronOutputs))
	expOutputs := make([]*rlwe.Ciphertext, len(neuronOutputs))

	for i, output := range neuronOutputs {
		fmt.Printf("  计算exp(神经元%d)...\n", i)
		expResult, err := sp.expActivation.Apply(output, evaluator, params)
		if err != nil {
			return nil, fmt.Errorf("计算exp失败(神经元%d): %v", i, err)
		}
		expOutputs[i] = expResult
	}

	// 步骤2：计算所有指数的和
	fmt.Println("\n步骤2：计算sum(exp(O_i))")
	sumExp := expOutputs[0].CopyNew()

	for i := 1; i < len(expOutputs); i++ {
		fmt.Printf("  累加神经元%d的exp值...\n", i)
		var err error
		sumExp, err = evaluator.AddNew(sumExp, expOutputs[i])
		if err != nil {
			return nil, fmt.Errorf("累加exp失败: %v", err)
		}
	}

	// 步骤3：计算倒数 1/sum(exp)
	fmt.Println("\n步骤3：计算1/sum(exp(O_i))")
	recipSum, err := sp.reciprocalActivation.Apply(sumExp, evaluator, params)
	if err != nil {
		return nil, fmt.Errorf("计算倒数失败: %v", err)
	}

	// 步骤4：计算softmax = exp(O_i) * (1/sum(exp))
	fmt.Println("\n步骤4：计算softmax概率")

	// 检查expOutputs和recipSum的层级是否足够进行乘法和rescale
	minLevel := params.MaxLevel()
	for _, ct := range expOutputs {
		if ct.Level() < minLevel {
			minLevel = ct.Level()
		}
	}
	if recipSum.Level() < minLevel {
		minLevel = recipSum.Level()
	}

	fmt.Printf("🔍 步骤4前层级检查: exp输出最低层级=%d, 倒数层级=%d, 需要至少2个层级进行乘法和rescale\n",
		minLevel, recipSum.Level())

	if minLevel < 2 {
		return nil, fmt.Errorf("层级不足进行softmax计算: 最低层级=%d, 需要至少2个层级", minLevel)
	}

	softmaxOutputs := make([]*rlwe.Ciphertext, len(expOutputs))

	for i, expOutput := range expOutputs {
		fmt.Printf("  计算softmax(神经元%d) = exp(O_%d) / sum...\n", i, i)

		// 乘以倒数
		result, err := evaluator.MulNew(expOutput, recipSum)
		if err != nil {
			return nil, fmt.Errorf("计算softmax失败(神经元%d): %v", i, err)
		}

		// Rescale以保持精度
		if err := evaluator.Rescale(result, result); err != nil {
			// 提供更详细的错误信息，包括层级信息
			return nil, fmt.Errorf("Rescale失败(神经元%d): %v (当前层级: %d/%d, 需要至少1个层级进行Rescale)",
				i, err, result.Level(), params.MaxLevel())
		}

		softmaxOutputs[i] = result
	}

	// 检查并输出最终softmax密文的层级状态
	fmt.Printf("\n🔍 Softmax计算后密文层级状态：\n")
	fmt.Println("序号 | 层级     | 缩放比例        | 状态")
	fmt.Println("-----|----------|----------------|----------")

	minFinalLevel := params.MaxLevel()
	totalLevels := 0

	for i, ct := range softmaxOutputs {
		level := ct.Level()
		scale := ct.Scale.Log2()

		if level < minFinalLevel {
			minFinalLevel = level
		}
		totalLevels += level

		// 计算层级状态
		ratio := float64(level) / float64(params.MaxLevel())
		var status string
		if ratio > 0.6 {
			status = "充足 ✅"
		} else if ratio > 0.3 {
			status = "中等 ⚠️"
		} else {
			status = "偏低 ❌"
		}

		fmt.Printf("%4d | %3d/%-3d | 2^%8.2f | %s\n",
			i, level, params.MaxLevel(), scale, status)
	}

	avgLevel := float64(totalLevels) / float64(len(softmaxOutputs))

	fmt.Println("-----|----------|----------------|----------")
	fmt.Printf("统计 | 最低: %d | 平均: %.1f | 最大: %d\n", minFinalLevel, avgLevel, params.MaxLevel())

	// 给出层级评估
	if minFinalLevel < params.MaxLevel()/3 {
		fmt.Printf("⚠️ 警告：最低层级过低（%d/%d），后续计算可能需要自举\n", minFinalLevel, params.MaxLevel())
	} else if minFinalLevel < params.MaxLevel()/2 {
		fmt.Printf("💡 提示：层级中等（%d/%d），建议监控后续计算\n", minFinalLevel, params.MaxLevel())
	} else {
		fmt.Printf("✅ 良好：层级充足（%d/%d），可继续后续计算\n", minFinalLevel, params.MaxLevel())
	}

	fmt.Printf("\n✓ Softmax计算完成，生成%d个概率分布\n", len(softmaxOutputs))

	return softmaxOutputs, nil
}

func (sp *SoftmaxProcessor) GetName() string {
	return fmt.Sprintf("Softmax(exp=%s, recip=%s)",
		sp.expActivation.GetName(), sp.reciprocalActivation.GetName())
}
