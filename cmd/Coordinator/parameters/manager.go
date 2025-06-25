package parameters

import (
	"MPHEDev/cmd/Coordinator/utils"
	"MPHEDev/pkg/setup"

	"github.com/tuneinsight/lattigo/v6/circuits/ckks/bootstrapping"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	lattigoUtils "github.com/tuneinsight/lattigo/v6/utils"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

// Manager 参数管理器
type Manager struct {
	params        ckks.Parameters
	paramsLiteral ckks.ParametersLiteral
	globalCRP     multiparty.PublicKeyGenCRP
	crpBytes      string

	// 刷新协议相关
	refreshCRS     *sampling.KeyedPRNG
	refreshCRSSeed string // 使用种子而不是序列化对象

	// 伽罗瓦密钥相关
	galEls          []uint64
	galoisCRPs      map[uint64]multiparty.GaloisKeyGenCRP
	galoisCRPsBytes map[uint64]string // base64编码的CRPs

	// 重线性化密钥相关
	rlkCRP      multiparty.RelinearizationKeyGenCRP
	rlkCRPBytes string
}

// NewManager 创建新的参数管理器
func NewManager() (*Manager, error) {
	params, err := setup.InitParameters()
	if err != nil {
		return nil, err
	}

	// 生成公钥CRP
	proto := multiparty.NewPublicKeyGenProtocol(params)
	crs, err := sampling.NewPRNG()
	if err != nil {
		return nil, err
	}
	crp := proto.SampleCRP(crs)
	crpRaw, err := utils.EncodeShare(crp)
	if err != nil {
		return nil, err
	}

	// 生成刷新CRS
	refreshCRS, err := sampling.NewKeyedPRNG([]byte("refresh_crs_seed_32_bytes_long"))
	if err != nil {
		return nil, err
	}
	// 使用种子而不是序列化对象
	refreshCRSSeed := utils.EncodeToBase64([]byte("refresh_crs_seed_32_bytes_long"))

	// 生成伽罗瓦元素和CRPs
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

	// 生成伽罗瓦密钥CRPs
	galoisProto := multiparty.NewGaloisKeyGenProtocol(params)
	galoisCRPs := make(map[uint64]multiparty.GaloisKeyGenCRP)
	galoisCRPsBytes := make(map[uint64]string)

	for _, galEl := range galEls {
		galoisCRP := galoisProto.SampleCRP(crs)
		galoisCRPs[galEl] = galoisCRP
		crpRaw, err := utils.EncodeShare(galoisCRP)
		if err != nil {
			return nil, err
		}
		galoisCRPsBytes[galEl] = utils.EncodeToBase64(crpRaw)
	}

	// 生成重线性化密钥CRP
	rlkProto := multiparty.NewRelinearizationKeyGenProtocol(params)
	rlkCRP := rlkProto.SampleCRP(crs)
	rlkCRPRaw, err := utils.EncodeShare(rlkCRP)
	if err != nil {
		return nil, err
	}

	return &Manager{
		params:          params,
		paramsLiteral:   params.ParametersLiteral(),
		globalCRP:       crp,
		crpBytes:        utils.EncodeToBase64(crpRaw),
		refreshCRS:      refreshCRS,
		refreshCRSSeed:  refreshCRSSeed,
		galEls:          galEls,
		galoisCRPs:      galoisCRPs,
		galoisCRPsBytes: galoisCRPsBytes,
		rlkCRP:          rlkCRP,
		rlkCRPBytes:     utils.EncodeToBase64(rlkCRPRaw),
	}, nil
}

// GetParams 获取所有参数
func (pm *Manager) GetParams() (ckks.ParametersLiteral, string, []uint64, map[uint64]string, string, string) {
	return pm.paramsLiteral, pm.crpBytes, pm.galEls, pm.galoisCRPsBytes, pm.rlkCRPBytes, pm.refreshCRSSeed
}

// GetCKKSParams 获取CKKS参数
func (pm *Manager) GetCKKSParams() ckks.Parameters {
	return pm.params
}

// GetParamsLiteral 获取参数字面量
func (pm *Manager) GetParamsLiteral() ckks.ParametersLiteral {
	return pm.paramsLiteral
}

// GetGlobalCRP 获取全局CRP
func (pm *Manager) GetGlobalCRP() multiparty.PublicKeyGenCRP {
	return pm.globalCRP
}

// GetGalEls 获取伽罗瓦元素列表
func (pm *Manager) GetGalEls() []uint64 {
	return pm.galEls
}

// GetGaloisCRPs 获取伽罗瓦密钥CRPs
func (pm *Manager) GetGaloisCRPs() map[uint64]multiparty.GaloisKeyGenCRP {
	return pm.galoisCRPs
}

// GetRelinearizationCRP 获取重线性化密钥CRP
func (pm *Manager) GetRelinearizationCRP() multiparty.RelinearizationKeyGenCRP {
	return pm.rlkCRP
}

// GetRefreshCRS 获取刷新CRS
func (pm *Manager) GetRefreshCRS() *sampling.KeyedPRNG {
	return pm.refreshCRS
}

// GetRefreshCRSBytes 获取刷新CRS的base64编码
func (pm *Manager) GetRefreshCRSBytes() string {
	return pm.refreshCRSSeed
}
