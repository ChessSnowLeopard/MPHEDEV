package homomorphic

import (
	"fmt"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// MaskReorganizer 掩码重组器
type MaskReorganizer struct {
	K     int // 每个密文包含的神经元数
	N     int // 每个段的样本数
	Slots int // 总槽数
}

// NewMaskReorganizer 创建掩码重组器
func NewMaskReorganizer(k, n, slots int) *MaskReorganizer {
	return &MaskReorganizer{
		K:     k,
		N:     n,
		Slots: slots,
	}
}

// GenerateMask 生成掩码
// segmentIdx: 要保留的段索引 (0 到 k-1)
func (mr *MaskReorganizer) GenerateMask(segmentIdx int, params ckks.Parameters, encoder *ckks.Encoder) (*rlwe.Plaintext, error) {
	mask := make([]complex128, mr.Slots)

	// 在第segmentIdx段设置为1
	start := segmentIdx * mr.N
	end := start + mr.N

	for i := start; i < end && i < mr.Slots; i++ {
		mask[i] = complex(1.0, 0)
	}

	// 编码掩码
	pt := ckks.NewPlaintext(params, params.MaxLevel())
	if err := encoder.Encode(mask, pt); err != nil {
		return nil, fmt.Errorf("编码掩码失败: %v", err)
	}

	return pt, nil
}

// ReorganizeOutputs 使用掩码重组神经元输出
// neuronOutputs: 每个神经元的输出密文（格式：[n个样本输出 | 重复k次]）
// 返回: 重组后的密文（格式：[样本0神经元0-k, 样本1神经元0-k, ..., 样本n-1神经元0-k | ...]）
func (mr *MaskReorganizer) ReorganizeOutputs(
	neuronOutputs []*rlwe.Ciphertext,
	params ckks.Parameters,
	encoder *ckks.Encoder,
	evaluator *ckks.Evaluator) ([]*rlwe.Ciphertext, error) {

	numNeurons := len(neuronOutputs)
	numOutputVectors := (numNeurons + mr.K - 1) / mr.K
	reorganized := make([]*rlwe.Ciphertext, numOutputVectors)

	fmt.Printf("\n开始重组 %d 个神经元输出为 %d 个向量...\n", numNeurons, numOutputVectors)

	for vecIdx := 0; vecIdx < numOutputVectors; vecIdx++ {
		var result *rlwe.Ciphertext

		// 处理这个输出向量中的k个神经元
		for j := 0; j < mr.K; j++ {
			neuronIdx := vecIdx*mr.K + j
			if neuronIdx >= numNeurons {
				break
			}

			// 生成掩码
			mask, err := mr.GenerateMask(j, params, encoder)
			if err != nil {
				return nil, fmt.Errorf("生成掩码失败: %v", err)
			}

			// 应用掩码：提取第j段
			masked, err := evaluator.MulNew(neuronOutputs[neuronIdx], mask)
			if err != nil {
				return nil, fmt.Errorf("掩码乘法失败: %v", err)
			}
			// 乘法后立即rescale
			if err := evaluator.Rescale(masked, masked); err != nil {
				return nil, fmt.Errorf("Rescale失败: %v", err)
			}

			// 累加到结果
			if result == nil {
				result = masked
			} else {
				result, err = evaluator.AddNew(result, masked)
				if err != nil {
					return nil, fmt.Errorf("累加失败: %v", err)
				}
			}
		}

		reorganized[vecIdx] = result
		fmt.Printf("✓ 完成第 %d 个向量的重组\n", vecIdx+1)
	}

	return reorganized, nil
}

// PrintReorganizationInfo 打印重组信息
func (mr *MaskReorganizer) PrintReorganizationInfo(numNeurons int) {
	fmt.Printf("\n=== 掩码重组配置 ===\n")
	fmt.Printf("• 每密文神经元数(k): %d\n", mr.K)
	fmt.Printf("• 每段样本数(n): %d\n", mr.N)
	fmt.Printf("• 总槽数: %d\n", mr.Slots)
	fmt.Printf("• 输入神经元数: %d\n", numNeurons)
	fmt.Printf("• 输出向量数: %d\n", (numNeurons+mr.K-1)/mr.K)

	fmt.Printf("\n重组模式:\n")
	fmt.Printf("输入格式（每个神经元）:\n")
	fmt.Printf("  神经元i: [样本0_输出i, 样本1_输出i, ..., 样本n-1_输出i | 重复k次]\n")

	fmt.Printf("\n输出格式（重组后）:\n")
	fmt.Printf("  向量0: [样本0_神经元0, ..., 样本n-1_神经元0 | ")
	fmt.Printf("样本0_神经元1, ..., 样本n-1_神经元1 | ... | ")
	fmt.Printf("样本0_神经元k-1, ..., 样本n-1_神经元k-1]\n")

	fmt.Printf("  向量1: [样本0_神经元k, ..., 样本n-1_神经元k | ")
	fmt.Printf("样本0_神经元k+1, ..., 样本n-1_神经元k+1 | ...]\n")
	fmt.Printf("  ...\n")
	fmt.Printf("=====================\n")
}
