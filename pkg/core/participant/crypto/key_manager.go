package crypto

import (
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// KeyManager 密钥管理
type KeyManager struct {
	Params          ckks.Parameters
	TotalGaloisKeys int
	PubKey          *rlwe.PublicKey
	RelineKey       *rlwe.RelinearizationKey
	GaloisKeys      []*rlwe.GaloisKey
	Sk              *rlwe.SecretKey
	AggregatedSK    *rlwe.SecretKey // 新增：协调器聚合的协同私钥（仅测试模式）
	Evaluator       *ckks.Evaluator
	Encoder         *ckks.Encoder
}

// NewKeyManager 创建新的密钥管理器
func NewKeyManager() *KeyManager {
	return &KeyManager{}
}

// SetParams 设置参数
func (km *KeyManager) SetParams(params ckks.Parameters) {
	km.Params = params
	km.Encoder = ckks.NewEncoder(params)
}

// SetSecretKey 设置私钥
func (km *KeyManager) SetSecretKey(sk *rlwe.SecretKey) {
	km.Sk = sk
}

// SetPublicKey 设置公钥
func (km *KeyManager) SetPublicKey(pk *rlwe.PublicKey) {
	km.PubKey = pk
}

// SetRelinearizationKey 设置重线性化密钥
func (km *KeyManager) SetRelinearizationKey(rlk *rlwe.RelinearizationKey) {
	km.RelineKey = rlk
}

// SetGaloisKeys 设置伽罗瓦密钥
func (km *KeyManager) SetGaloisKeys(galoisKeys []*rlwe.GaloisKey) {
	km.GaloisKeys = galoisKeys
	km.TotalGaloisKeys = len(galoisKeys)
}

// SetEvaluator 设置Evaluator
func (km *KeyManager) SetEvaluator(evaluator *ckks.Evaluator) {
	km.Evaluator = evaluator
}

// GetEvaluator 获取Evaluator
func (km *KeyManager) GetEvaluator() *ckks.Evaluator {
	return km.Evaluator
}

// SetEncoder 设置Encoder
func (km *KeyManager) SetEncoder(encoder *ckks.Encoder) {
	km.Encoder = encoder
}

// GetEncoder 获取Encoder
func (km *KeyManager) GetEncoder() *ckks.Encoder {
	return km.Encoder
}

// GetParams 获取参数
func (km *KeyManager) GetParams() ckks.Parameters {
	return km.Params
}

// GetSecretKey 获取私钥
func (km *KeyManager) GetSecretKey() *rlwe.SecretKey {
	return km.Sk
}

// GetPublicKey 获取公钥
func (km *KeyManager) GetPublicKey() *rlwe.PublicKey {
	return km.PubKey
}

// GetRelinearizationKey 获取重线性化密钥
func (km *KeyManager) GetRelinearizationKey() *rlwe.RelinearizationKey {
	return km.RelineKey
}

// GetGaloisKeys 获取伽罗瓦密钥
func (km *KeyManager) GetGaloisKeys() []*rlwe.GaloisKey {
	return km.GaloisKeys
}

// SetAggregatedSecretKey 设置聚合的协同私钥（仅测试模式）
func (km *KeyManager) SetAggregatedSecretKey(sk *rlwe.SecretKey) {
	km.AggregatedSK = sk
}

// GetAggregatedSecretKey 获取聚合的协同私钥（仅测试模式）
func (km *KeyManager) GetAggregatedSecretKey() *rlwe.SecretKey {
	return km.AggregatedSK
}

// GetEncryptor 获取加密器
func (km *KeyManager) GetEncryptor() *rlwe.Encryptor {
	if km.PubKey == nil {
		return nil
	}
	return ckks.NewEncryptor(km.Params, km.PubKey)
}

// IsReady 检查密钥是否准备就绪
func (km *KeyManager) IsReady() bool {
	return km.Sk != nil && km.PubKey != nil && km.RelineKey != nil
}
