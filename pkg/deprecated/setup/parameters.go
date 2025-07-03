package setup

import (
	"MPHEDev/pkg/participant"

	"github.com/tuneinsight/lattigo/v6/circuits/ckks/bootstrapping"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

// InitParameters 初始化CKKS参数
func InitParameters() (ckks.Parameters, error) {
	return ckks.NewParametersFromLiteral(
		ckks.ParametersLiteral{
			LogN:            14,
			LogQ:            []int{55, 45, 45, 45, 45, 45, 45, 45},
			LogP:            []int{61, 61, 61},
			LogDefaultScale: 45,
			Xs:              ring.Ternary{H: 192},
		})
}

// InitializeSystem 初始化整个系统
func InitializeSystem(N int) (ckks.Parameters, []*participant.Party, *participant.Cloud,
	[]uint64, *sampling.KeyedPRNG, error) {
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

	// 设置自举参数获取伽罗瓦元素
	// 因此后续想要相应的galois元素需要修改此处的逻辑
	btpParametersLit := bootstrapping.ParametersLiteral{
		LogN: utils.Pointy(params.LogN()),
		LogP: params.LogPi(),
		Xs:   params.Xs(),
	}
	btpParams, err := bootstrapping.NewParametersFromLiteral(params, btpParametersLit)
	if err != nil {
		return params, nil, nil, nil, nil, err
	}
	galEls := btpParams.GaloisElements(params)

	//这里是原框架下的部分，现在不需要云端，所以注释掉
	/* 创建云端实例
	cloud := participant.NewCloud(params, N, galEls, crs)

	// 创建参与方实例
	parties := make([]*participant.Party, N)
	kg := rlwe.NewKeyGenerator(params)

	for i := range parties {
		parties[i] = participant.NewParty(i, params, kg)
	}*/

	return params, nil, nil, galEls, crs, nil
}

// GenerateAggregatedSecretKey 生成聚合私钥（仅用于测试）
func GenerateAggregatedSecretKey(params ckks.Parameters, parties []*participant.Party) *rlwe.SecretKey {
	skAgg := rlwe.NewSecretKey(params)
	for _, p := range parties {
		params.RingQP().Add(skAgg.Value, p.Sk.Value, skAgg.Value)
	}
	return skAgg
}
