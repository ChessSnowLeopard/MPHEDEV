package keys

import (
	"MPHEDev/pkg/core/coordinator/utils"
	"fmt"
	"sync"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// Manager 密钥管理器
type Manager struct {
	params    ckks.Parameters
	expectedN int
	mu        sync.RWMutex

	// 公钥相关
	publicKeyShares map[int][]byte
	globalPK        *rlwe.PublicKey

	// 私钥相关
	secretKeyShares map[int][]byte
	skAgg           *rlwe.SecretKey

	// 伽罗瓦密钥相关
	galoisKeyShares map[uint64]map[int][]byte // galEl -> participantID -> share
	galoisKeys      []*rlwe.GaloisKey
	galoisProto     multiparty.GaloisKeyGenProtocol

	// 重线性化密钥相关
	rlkShare1Map        map[int][]byte                         // participantID -> share1
	rlkShare2Map        map[int][]byte                         // participantID -> share2
	rlkShare1Aggregated *multiparty.RelinearizationKeyGenShare // 聚合后的第一轮份额
	rlk                 *rlwe.RelinearizationKey
	rlkProto            multiparty.RelinearizationKeyGenProtocol
	rlkRound            int // 当前轮次：1或2
}

// NewManager 创建新的密钥管理器
func NewManager(params ckks.Parameters, expectedN int) *Manager {
	return &Manager{
		params:              params,
		expectedN:           expectedN,
		publicKeyShares:     make(map[int][]byte),
		secretKeyShares:     make(map[int][]byte),
		galoisKeyShares:     make(map[uint64]map[int][]byte),
		galoisKeys:          make([]*rlwe.GaloisKey, 0),
		galoisProto:         multiparty.NewGaloisKeyGenProtocol(params),
		rlkShare1Map:        make(map[int][]byte),
		rlkShare2Map:        make(map[int][]byte),
		rlkShare1Aggregated: nil,
		rlk:                 nil,
		rlkProto:            multiparty.NewRelinearizationKeyGenProtocol(params),
		rlkRound:            1,
	}
}

// AddPublicKeyShare 添加公钥份额
func (km *Manager) AddPublicKeyShare(participantID int, data []byte) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	km.publicKeyShares[participantID] = data

	// 只在达到预期数量时输出汇总信息
	if len(km.publicKeyShares) == km.expectedN {
		fmt.Printf("✓ 所有参与方公钥份额已收集完成 (%d/%d)\n", len(km.publicKeyShares), km.expectedN)
	} else if len(km.publicKeyShares)%5 == 0 || len(km.publicKeyShares) == 1 {
		// 每5个或第1个时输出进度
		fmt.Printf("公钥份额收集进度: %d/%d\n", len(km.publicKeyShares), km.expectedN)
	}
	return nil
}

// AddSecretKey 添加私钥
func (km *Manager) AddSecretKey(participantID int, data []byte) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	km.secretKeyShares[participantID] = data

	// 只在达到预期数量时输出汇总信息
	if len(km.secretKeyShares) == km.expectedN {
		fmt.Printf("✓ 所有参与方私钥已收集完成 (%d/%d)\n", len(km.secretKeyShares), km.expectedN)
	} else if len(km.secretKeyShares)%5 == 0 || len(km.secretKeyShares) == 1 {
		// 每5个或第1个时输出进度
		fmt.Printf("私钥收集进度: %d/%d\n", len(km.secretKeyShares), km.expectedN)
	}
	return nil
}

// AddGaloisKeyShare 添加伽罗瓦密钥份额
func (km *Manager) AddGaloisKeyShare(participantID int, galEl uint64, data []byte) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	if km.galoisKeyShares[galEl] == nil {
		km.galoisKeyShares[galEl] = make(map[int][]byte)
	}

	km.galoisKeyShares[galEl][participantID] = data

	// 只在达到预期数量时输出汇总信息
	if len(km.galoisKeyShares[galEl]) == km.expectedN {
		fmt.Printf("✓ 伽罗瓦密钥份额收集完成 (galEl: %d, %d/%d)\n", galEl, len(km.galoisKeyShares[galEl]), km.expectedN)
	} else if len(km.galoisKeyShares[galEl])%5 == 0 || len(km.galoisKeyShares[galEl]) == 1 {
		// 每5个或第1个时输出进度
		fmt.Printf("伽罗瓦密钥份额收集进度 (galEl: %d): %d/%d\n", galEl, len(km.galoisKeyShares[galEl]), km.expectedN)
	}
	return nil
}

// AddRelinearizationKeyShare 添加重线性化密钥份额
func (km *Manager) AddRelinearizationKeyShare(participantID int, round int, data []byte) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	if round == 1 {
		km.rlkShare1Map[participantID] = data

		// 只在达到预期数量时输出汇总信息
		if len(km.rlkShare1Map) == km.expectedN {
			fmt.Printf("✓ 重线性化密钥第一轮份额收集完成 (%d/%d)\n", len(km.rlkShare1Map), km.expectedN)
		} else if len(km.rlkShare1Map)%5 == 0 || len(km.rlkShare1Map) == 1 {
			// 每5个或第1个时输出进度
			fmt.Printf("重线性化密钥第一轮份额收集进度: %d/%d\n", len(km.rlkShare1Map), km.expectedN)
		}
	} else if round == 2 {
		km.rlkShare2Map[participantID] = data

		// 只在达到预期数量时输出汇总信息
		if len(km.rlkShare2Map) == km.expectedN {
			fmt.Printf("✓ 重线性化密钥第二轮份额收集完成 (%d/%d)\n", len(km.rlkShare2Map), km.expectedN)
		} else if len(km.rlkShare2Map)%5 == 0 || len(km.rlkShare2Map) == 1 {
			// 每5个或第1个时输出进度
			fmt.Printf("重线性化密钥第二轮份额收集进度: %d/%d\n", len(km.rlkShare2Map), km.expectedN)
		}
	} else {
		return fmt.Errorf("无效的轮次: %d", round)
	}

	return nil
}

// GetRelinearizationKeyRound1Aggregated 获取聚合后的第一轮重线性化密钥份额
func (km *Manager) GetRelinearizationKeyRound1Aggregated() (string, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if km.rlkShare1Aggregated == nil {
		return "", fmt.Errorf("第一轮份额尚未聚合")
	}

	shareBytes, err := utils.EncodeShare(*km.rlkShare1Aggregated)
	if err != nil {
		return "", err
	}

	return utils.EncodeToBase64(shareBytes), nil
}

// GetGlobalPK 获取全局公钥
func (km *Manager) GetGlobalPK() *rlwe.PublicKey {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.globalPK
}

// GetAggregatedSecretKey 获取聚合私钥
func (km *Manager) GetAggregatedSecretKey() *rlwe.SecretKey {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.skAgg
}

// GetGaloisKeys 获取伽罗瓦密钥列表
func (km *Manager) GetGaloisKeys() []*rlwe.GaloisKey {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.galoisKeys
}

// GetRelinearizationKey 获取重线性化密钥
func (km *Manager) GetRelinearizationKey() *rlwe.RelinearizationKey {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.rlk
}

// GetExpectedN 获取期望的参与方数量
func (km *Manager) GetExpectedN() int {
	return km.expectedN
}

// GetParams 获取CKKS参数
func (km *Manager) GetParams() ckks.Parameters {
	return km.params
}

// GetGaloisProto 获取伽罗瓦密钥协议
func (km *Manager) GetGaloisProto() multiparty.GaloisKeyGenProtocol {
	return km.galoisProto
}

// GetRelinearizationProto 获取重线性化密钥协议
func (km *Manager) GetRelinearizationProto() multiparty.RelinearizationKeyGenProtocol {
	return km.rlkProto
}

// SetGlobalPK 设置全局公钥
func (km *Manager) SetGlobalPK(pk *rlwe.PublicKey) {
	km.mu.Lock()
	defer km.mu.Unlock()
	km.globalPK = pk
}

// SetAggregatedSecretKey 设置聚合私钥
func (km *Manager) SetAggregatedSecretKey(sk *rlwe.SecretKey) {
	km.mu.Lock()
	defer km.mu.Unlock()
	km.skAgg = sk
}

// AddGaloisKey 添加伽罗瓦密钥
func (km *Manager) AddGaloisKey(gk *rlwe.GaloisKey) {
	km.mu.Lock()
	defer km.mu.Unlock()
	km.galoisKeys = append(km.galoisKeys, gk)
}

// SetRelinearizationKey 设置重线性化密钥
func (km *Manager) SetRelinearizationKey(rlk *rlwe.RelinearizationKey) {
	km.mu.Lock()
	defer km.mu.Unlock()
	km.rlk = rlk
}

// SetRelinearizationShare1Aggregated 设置聚合后的第一轮重线性化密钥份额
func (km *Manager) SetRelinearizationShare1Aggregated(share *multiparty.RelinearizationKeyGenShare) {
	km.mu.Lock()
	defer km.mu.Unlock()
	km.rlkShare1Aggregated = share
}

// GetPublicKeyShares 获取公钥份额
func (km *Manager) GetPublicKeyShares() map[int][]byte {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.publicKeyShares
}

// GetSecretKeyShares 获取私钥份额
func (km *Manager) GetSecretKeyShares() map[int][]byte {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.secretKeyShares
}

// GetGaloisKeyShares 获取伽罗瓦密钥份额
func (km *Manager) GetGaloisKeyShares() map[uint64]map[int][]byte {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.galoisKeyShares
}

// GetRelinearizationShare1Map 获取第一轮重线性化密钥份额
func (km *Manager) GetRelinearizationShare1Map() map[int][]byte {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.rlkShare1Map
}

// GetRelinearizationShare2Map 获取第二轮重线性化密钥份额
func (km *Manager) GetRelinearizationShare2Map() map[int][]byte {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.rlkShare2Map
}

// GetRelinearizationShare1Aggregated 获取聚合后的第一轮重线性化密钥份额
func (km *Manager) GetRelinearizationShare1Aggregated() *multiparty.RelinearizationKeyGenShare {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.rlkShare1Aggregated
}
