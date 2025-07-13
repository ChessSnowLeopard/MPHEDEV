package computation

import (
	"MPHEDev/pkg/core/participant/interfaces"
	"fmt"
	"math"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// MaskReorganizer 掩码重组器
type MaskReorganizer struct {
	K               int // 每密文特征数
	ImagesPerVector int // 每向量图像数
	Slots           int // 密文槽数
}

// NewMaskReorganizer 创建掩码重组器
func NewMaskReorganizer(k, imagesPerVector, slots int) *MaskReorganizer {
	return &MaskReorganizer{
		K:               k,
		ImagesPerVector: imagesPerVector,
		Slots:           slots,
	}
}

// ReorganizeOutputs 重组输出
func (mr *MaskReorganizer) ReorganizeOutputs(
	neuronOutputs []*rlwe.Ciphertext,
	params ckks.Parameters,
	encoder *ckks.Encoder,
	evaluator *ckks.Evaluator) ([]*rlwe.Ciphertext, error) {

	fmt.Println("\n=== 开始输出重组 ===")
	fmt.Printf("• 神经元输出数: %d\n", len(neuronOutputs))
	fmt.Printf("• 每密文特征数(k): %d\n", mr.K)
	fmt.Printf("• 每向量图像数(n): %d\n", mr.ImagesPerVector)
	fmt.Printf("• 密文槽数(s): %d\n", mr.Slots)

	// 计算需要的向量数
	numVectors := int(math.Ceil(float64(len(neuronOutputs)) / float64(mr.K)))
	fmt.Printf("• 需要的向量数: %d\n", numVectors)

	// 创建重组后的向量
	reorganizedVectors := make([]*rlwe.Ciphertext, numVectors)

	// 对每个向量进行处理
	for vecIdx := 0; vecIdx < numVectors; vecIdx++ {
		fmt.Printf("\n处理向量 %d/%d...\n", vecIdx+1, numVectors)

		// 计算当前向量处理的神经元范围
		neuronStart := vecIdx * mr.K
		neuronEnd := interfaces.Min(neuronStart+mr.K, len(neuronOutputs))
		actualK := neuronEnd - neuronStart

		fmt.Printf("  神经元范围: [%d-%d), 实际特征数: %d\n", neuronStart, neuronEnd, actualK)

		// 创建掩码向量
		maskVector := make([]complex128, mr.Slots)
		for i := 0; i < mr.Slots; i++ {
			maskVector[i] = complex(0.0, 0)
		}

		// 设置掩码
		for i := 0; i < actualK; i++ {
			for j := 0; j < mr.ImagesPerVector; j++ {
				idx := j*mr.K + i
				if idx < mr.Slots {
					maskVector[idx] = complex(1.0, 0)
				}
			}
		}

		// 编码掩码
		maskPt := ckks.NewPlaintext(params, params.MaxLevel())
		if err := encoder.Encode(maskVector, maskPt); err != nil {
			return nil, fmt.Errorf("编码掩码失败: %v", err)
		}

		// 应用掩码到第一个神经元输出
		var result *rlwe.Ciphertext
		var err error
		if neuronStart < len(neuronOutputs) {
			result, err = evaluator.MulNew(neuronOutputs[neuronStart], maskPt)
			if err != nil {
				return nil, fmt.Errorf("应用掩码失败: %v", err)
			}
			if err := evaluator.Rescale(result, result); err != nil {
				return nil, fmt.Errorf("Rescale失败: %v", err)
			}

			// 累加其他神经元输出
			for i := neuronStart + 1; i < neuronEnd; i++ {
				masked, err := evaluator.MulNew(neuronOutputs[i], maskPt)
				if err != nil {
					return nil, fmt.Errorf("应用掩码失败: %v", err)
				}
				if err := evaluator.Rescale(masked, masked); err != nil {
					return nil, fmt.Errorf("Rescale失败: %v", err)
				}
				result, err = evaluator.AddNew(result, masked)
				if err != nil {
					return nil, fmt.Errorf("累加失败: %v", err)
				}
			}
		} else {
			// 如果没有神经元输出，返回错误
			return nil, fmt.Errorf("没有可用的神经元输出进行重组")
		}

		reorganizedVectors[vecIdx] = result
		fmt.Printf("  ✓ 向量 %d 重组完成\n", vecIdx+1)
	}

	fmt.Printf("\n✓ 输出重组完成，生成 %d 个重组向量\n", len(reorganizedVectors))
	return reorganizedVectors, nil
}

// CreateMaskForNeuron 为特定神经元创建掩码
func (mr *MaskReorganizer) CreateMaskForNeuron(
	neuronIdx int,
	params ckks.Parameters,
	encoder *ckks.Encoder) (*rlwe.Plaintext, error) {

	// 计算神经元在向量中的位置
	_ = neuronIdx / mr.K // 暂时不使用vectorIdx
	positionInVector := neuronIdx % mr.K

	// 创建掩码
	mask := make([]complex128, mr.Slots)
	for i := 0; i < mr.Slots; i++ {
		mask[i] = complex(0.0, 0)
	}

	// 设置对应位置的掩码
	for j := 0; j < mr.ImagesPerVector; j++ {
		idx := j*mr.K + positionInVector
		if idx < mr.Slots {
			mask[idx] = complex(1.0, 0)
		}
	}

	// 编码掩码
	maskPt := ckks.NewPlaintext(params, params.MaxLevel())
	if err := encoder.Encode(mask, maskPt); err != nil {
		return nil, fmt.Errorf("编码掩码失败: %v", err)
	}

	return maskPt, nil
}

// ApplyMask 应用掩码到密文
func (mr *MaskReorganizer) ApplyMask(
	ct *rlwe.Ciphertext,
	mask *rlwe.Plaintext,
	evaluator *ckks.Evaluator) (*rlwe.Ciphertext, error) {

	// 应用掩码
	result, err := evaluator.MulNew(ct, mask)
	if err != nil {
		return nil, fmt.Errorf("应用掩码失败: %v", err)
	}

	// Rescale
	if err := evaluator.Rescale(result, result); err != nil {
		return nil, fmt.Errorf("Rescale失败: %v", err)
	}

	return result, nil
}

// PrintReorganizationInfo 打印重组信息
func (mr *MaskReorganizer) PrintReorganizationInfo() {
	fmt.Println("\n=== 掩码重组器配置 ===")
	fmt.Printf("• 每密文特征数(k): %d\n", mr.K)
	fmt.Printf("• 每向量图像数(n): %d\n", mr.ImagesPerVector)
	fmt.Printf("• 密文槽数(s): %d\n", mr.Slots)
	fmt.Printf("• 向量大小: %d x %d\n", mr.ImagesPerVector, mr.K)
	fmt.Println("=====================")
}
