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
}

// NewKeyManager 创建新的密钥管理器
func NewKeyManager() *KeyManager {
	return &KeyManager{}
}

// SetParams 设置参数
func (km *KeyManager) SetParams(params ckks.Parameters) {
	km.Params = params
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

// IsReady 检查密钥是否准备就绪
func (km *KeyManager) IsReady() bool {
	return km.Sk != nil && km.PubKey != nil && km.RelineKey != nil
}
