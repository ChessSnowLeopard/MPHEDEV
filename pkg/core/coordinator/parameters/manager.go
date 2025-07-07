package parameters

import (
	"MPHEDev/pkg/core/coordinator/utils"
	"encoding/json"
	"fmt"

	"github.com/tuneinsight/lattigo/v6/circuits/ckks/bootstrapping"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	lattigoUtils "github.com/tuneinsight/lattigo/v6/utils"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

// Manager 参数管理器
type Manager struct {
	params             ckks.Parameters
	paramsLiteral      ckks.ParametersLiteral
	paramsLiteralBytes string // base64编码的参数字面量

	// 统一的CRS种子
	commonCRSSeed string // 统一的CRS种子，用于所有参与方生成相同的CRP

	// 内部CRP（不通过网络传输，仅用于协调器内部聚合）
	globalCRP  multiparty.PublicKeyGenCRP
	galoisCRPs map[uint64]multiparty.GaloisKeyGenCRP
	rlkCRP     multiparty.RelinearizationKeyGenCRP

	// 伽罗瓦元素
	galEls []uint64

	// 数据集划分类型
	dataSplitType string
}

func initCKKSParameters() (ckks.Parameters, error) {
	originalParams := ckks.ParametersLiteral{
		LogN:            14,
		LogQ:            []int{55, 45, 45, 45, 45, 45, 45, 45},
		LogP:            []int{61},
		LogDefaultScale: 45,
		RingType:        ring.Standard,
	}

	fmt.Printf("输入参数: LogN=%d, LogQ=%v, LogP=%v\n",
		originalParams.LogN, originalParams.LogQ, originalParams.LogP)

	params, err := ckks.NewParametersFromLiteral(originalParams)
	if err != nil {
		fmt.Printf("[ERROR] 参数创建失败: %v\n", err)
		return params, err
	}

	//使用正确的方法验证参数
	fmt.Printf("[SUCCESS] 参数创建成功:\n")
	fmt.Printf("  实际LogN: %d\n", params.LogN())
	fmt.Printf("  Q模数数量: %d, 总位数: %.1f\n", params.QCount(), params.LogQ())
	fmt.Printf("  P模数数量: %d, 总位数: %.1f\n", params.PCount(), params.LogP())
	fmt.Printf("  默认精度: %d\n", params.LogDefaultScale())

	return params, nil
}

// NewManager 创建新的参数管理器
func NewManager(dataSplitType string) (*Manager, error) {
	params, err := initCKKSParameters()
	if err != nil {
		fmt.Printf("参数初始化失败: %v\n", err)
		return nil, err
	}
	// 生成伽罗瓦元素
	btpParametersLit := bootstrapping.ParametersLiteral{
		LogN: lattigoUtils.Pointy(params.LogN()),
		LogP: params.LogPi(),
		Xs:   params.Xs(),
	}
	btpParams, err := bootstrapping.NewParametersFromLiteral(params, btpParametersLit)
	if err != nil {
		return nil, err
	}
	galEls := btpParams.GaloisElements(params)

	// 生成一个统一的CRS种子（用于所有参与方生成相同的CRP）
	commonCRSSeed := []byte("common_crs_seed_32_bytes_long_for_all")
	commonCRSB64 := utils.EncodeToBase64(commonCRSSeed)

	// 使用统一种子生成CRP（协调器内部使用）
	crs, err := sampling.NewKeyedPRNG(commonCRSSeed)
	if err != nil {
		return nil, err
	}

	// 生成公钥CRP
	pubKeyProto := multiparty.NewPublicKeyGenProtocol(params)
	globalCRP := pubKeyProto.SampleCRP(crs)

	// 生成伽罗瓦密钥CRPs
	galoisProto := multiparty.NewGaloisKeyGenProtocol(params)
	galoisCRPs := make(map[uint64]multiparty.GaloisKeyGenCRP)
	for _, galEl := range galEls {
		galoisCRPs[galEl] = galoisProto.SampleCRP(crs)
	}

	// 生成重线性化密钥CRP
	rlkProto := multiparty.NewRelinearizationKeyGenProtocol(params)
	rlkCRP := rlkProto.SampleCRP(crs)

	// 获取参数字面量并序列化
	paramsLiteral := params.ParametersLiteral()
	jsonBytes, err := json.Marshal(paramsLiteral)
	if err != nil {
		return nil, fmt.Errorf("序列化参数字面量失败: %v", err)
	}
	paramsLiteralBytes := utils.EncodeToBase64(jsonBytes)

	return &Manager{
		params:             params,
		paramsLiteral:      paramsLiteral,
		paramsLiteralBytes: paramsLiteralBytes,
		commonCRSSeed:      commonCRSB64, // 统一的CRS种子
		globalCRP:          globalCRP,    // 协调器内部使用的CRP
		galoisCRPs:         galoisCRPs,   // 协调器内部使用的CRP
		rlkCRP:             rlkCRP,       // 协调器内部使用的CRP
		galEls:             galEls,
		dataSplitType:      dataSplitType,
	}, nil
}

// GetParams 获取所有参数
func (pm *Manager) GetParams() (string, []uint64, string, string) {
	return pm.paramsLiteralBytes, pm.galEls, pm.commonCRSSeed, pm.dataSplitType
}

// GetCKKSParams 获取CKKS参数
func (pm *Manager) GetCKKSParams() ckks.Parameters {
	return pm.params
}

// GetParamsLiteral 获取参数字面量
func (pm *Manager) GetParamsLiteral() ckks.ParametersLiteral {
	return pm.paramsLiteral
}

// GetGalEls 获取伽罗瓦元素列表
func (pm *Manager) GetGalEls() []uint64 {
	return pm.galEls
}

// GetDataSplitType 获取数据集划分类型
func (pm *Manager) GetDataSplitType() string {
	return pm.dataSplitType
}

// GetGlobalCRP 获取全局CRP
func (pm *Manager) GetGlobalCRP() multiparty.PublicKeyGenCRP {
	return pm.globalCRP
}

// GetGaloisCRPs 获取伽罗瓦密钥CRPs
func (pm *Manager) GetGaloisCRPs() map[uint64]multiparty.GaloisKeyGenCRP {
	return pm.galoisCRPs
}

// GetRelinearizationCRP 获取重线性化密钥CRP
func (pm *Manager) GetRelinearizationCRP() multiparty.RelinearizationKeyGenCRP {
	return pm.rlkCRP
}
