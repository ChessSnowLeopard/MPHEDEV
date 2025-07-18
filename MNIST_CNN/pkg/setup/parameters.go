package setup

import (
	"MNIST-CNN/pkg/participant"
	"fmt"

	"github.com/tuneinsight/lattigo/v6/circuits/ckks/bootstrapping"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

// RotationConfig 旋转配置
type RotationConfig struct {
	// 基本旋转位数
	BasicRotations []int
	// 批处理相关的旋转
	BatchRotations []int
	// 是否包含所有2的幂次旋转
	IncludePowerOfTwo bool
	// 最大旋转范围
	MaxRotation int
}

// InitParameters 初始化CKKS参数
func InitParameters() (ckks.Parameters, error) {
	return ckks.NewParametersFromLiteral(
		ckks.ParametersLiteral{
			LogN:            14,
			LogQ:            []int{55, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45},
			LogP:            []int{61, 61, 61},
			LogDefaultScale: 45,
			Xs:              ring.Ternary{H: 192},
		})
}

// InitializeSystem 初始化整个系统（保留原函数以兼容）
func InitializeSystem(N int) (ckks.Parameters, []*participant.Party, *participant.Cloud,
	[]uint64, *sampling.KeyedPRNG, error) {

	// 使用默认旋转配置
	defaultRotConfig := RotationConfig{
		BasicRotations:    []int{1, 2, 3, 4, 5, 10, 20},
		BatchRotations:    []int{64, 128, 256},
		IncludePowerOfTwo: false,
		MaxRotation:       256,
	}

	return InitializeSystemWithRotations(N, defaultRotConfig)
}

// InitializeSystemWithRotations 使用自定义旋转配置初始化系统
func InitializeSystemWithRotations(N int, rotConfig RotationConfig) (ckks.Parameters, []*participant.Party,
	*participant.Cloud, []uint64, *sampling.KeyedPRNG, error) {

	// 初始化参数
	params, err := InitParameters()
	if err != nil {
		return params, nil, nil, nil, nil, err
	}

	// 初始化伪随机数生成器
	crs, err := sampling.NewPRNG()
	if err != nil {
		return params, nil, nil, nil, nil, err
	}

	// 获取自举所需的伽罗瓦元素
	btpParametersLit := bootstrapping.ParametersLiteral{
		LogN: utils.Pointy(params.LogN()),
		LogP: params.LogPi(),
		Xs:   params.Xs(),
	}
	btpParams, err := bootstrapping.NewParametersFromLiteral(params, btpParametersLit)
	if err != nil {
		return params, nil, nil, nil, nil, err
	}
	bootstrapGalEls := btpParams.GaloisElements(params)

	// 生成旋转所需的伽罗瓦元素
	galElsMap := make(map[uint64]bool)

	// 添加自举元素
	fmt.Printf("添加 %d 个自举所需的伽罗瓦元素\n", len(bootstrapGalEls))
	for _, el := range bootstrapGalEls {
		galElsMap[el] = true
	}

	// 添加基本旋转
	fmt.Printf("添加基本旋转: %v\n", rotConfig.BasicRotations)
	for _, k := range rotConfig.BasicRotations {
		galElsMap[params.GaloisElement(k)] = true
		if k != 0 { // 避免重复添加0
			galElsMap[params.GaloisElement(-k)] = true // 同时添加反向旋转
		}
	}

	// 添加批处理旋转
	fmt.Printf("添加批处理旋转: %v\n", rotConfig.BatchRotations)
	for _, k := range rotConfig.BatchRotations {
		galElsMap[params.GaloisElement(k)] = true
		galElsMap[params.GaloisElement(-k)] = true
	}

	// 如果需要，添加所有2的幂次旋转
	if rotConfig.IncludePowerOfTwo {
		fmt.Printf("添加2的幂次旋转（最大到%d）\n", rotConfig.MaxRotation)
		for i := 0; i < params.LogN(); i++ {
			k := 1 << i
			if k <= rotConfig.MaxRotation {
				galElsMap[params.GaloisElement(k)] = true
				galElsMap[params.GaloisElement(-k)] = true
			}
		}
	}

	// 转换为切片
	galEls := make([]uint64, 0, len(galElsMap))
	for el := range galElsMap {
		galEls = append(galEls, el)
	}

	fmt.Printf("总共生成 %d 个伽罗瓦密钥（包括自举和旋转）\n", len(galEls))

	// 创建云端实例
	cloud := participant.NewCloud(params, N, galEls, crs)

	// 创建参与方实例
	parties := make([]*participant.Party, N)
	kg := rlwe.NewKeyGenerator(params)

	for i := range parties {
		parties[i] = participant.NewParty(i, params, kg)
	}

	return params, parties, cloud, galEls, crs, nil
}

// GenerateAggregatedSecretKey 生成聚合私钥（仅用于测试）
func GenerateAggregatedSecretKey(params ckks.Parameters, parties []*participant.Party) *rlwe.SecretKey {
	skAgg := rlwe.NewSecretKey(params)
	for _, p := range parties {
		params.RingQP().Add(skAgg.Value, p.Sk.Value, skAgg.Value)
	}
	return skAgg
}
