package keys

import (
	test "MPHEDev/pkg/testFunc"
	"fmt"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// Tester 密钥测试器
type Tester struct {
	keyManager *Manager
}

// NewTester 创建新的密钥测试器
func NewTester(keyManager *Manager) *Tester {
	return &Tester{
		keyManager: keyManager,
	}
}

// TestAllKeys 测试所有密钥
func (t *Tester) TestAllKeys(params ckks.Parameters, galEls []uint64) error {
	fmt.Println("\n========== 开始密钥测试 ==========")

	// 检查密钥是否准备就绪
	if t.keyManager.GetGlobalPK() == nil {
		return fmt.Errorf("全局公钥未准备就绪")
	}
	if t.keyManager.GetAggregatedSecretKey() == nil {
		return fmt.Errorf("聚合私钥未准备就绪")
	}
	if t.keyManager.GetRelinearizationKey() == nil {
		return fmt.Errorf("重线性化密钥未准备就绪")
	}
	if len(t.keyManager.GetGaloisKeys()) == 0 {
		return fmt.Errorf("伽罗瓦密钥未准备就绪")
	}

	// 创建编码器和加解密器
	encoder := ckks.NewEncoder(params)
	encryptor := ckks.NewEncryptor(params, t.keyManager.GetGlobalPK())
	decryptorAgg := ckks.NewDecryptor(params, t.keyManager.GetAggregatedSecretKey())

	// 创建评估密钥集
	evk := rlwe.NewMemEvaluationKeySet(t.keyManager.GetRelinearizationKey(), t.keyManager.GetGaloisKeys()...)

	// 测试公钥
	fmt.Println("\n--- 测试公钥功能 ---")
	test.TestPublicKey(params, encoder, encryptor, decryptorAgg)

	// 测试重线性化密钥
	fmt.Println("\n--- 测试重线性化密钥功能 ---")
	test.TestRelinearizationKey(params, evk, encoder, encryptor, decryptorAgg)

	// 测试伽罗瓦密钥
	fmt.Println("\n--- 测试伽罗瓦密钥功能 ---")
	test.TestGaloisKeys(params, t.keyManager.GetExpectedN(), evk, galEls, t.keyManager.GetAggregatedSecretKey())

	fmt.Println("\n========== 所有密钥测试完成 ==========")
	return nil
}

// TestPublicKeyOnly 仅测试公钥
func (t *Tester) TestPublicKeyOnly(params ckks.Parameters) error {
	if t.keyManager.GetGlobalPK() == nil || t.keyManager.GetAggregatedSecretKey() == nil {
		return fmt.Errorf("公钥或私钥未准备就绪")
	}

	encoder := ckks.NewEncoder(params)
	encryptor := ckks.NewEncryptor(params, t.keyManager.GetGlobalPK())
	decryptorAgg := ckks.NewDecryptor(params, t.keyManager.GetAggregatedSecretKey())

	test.TestPublicKey(params, encoder, encryptor, decryptorAgg)
	return nil
}

// TestRelinearizationKeyOnly 仅测试重线性化密钥
func (t *Tester) TestRelinearizationKeyOnly(params ckks.Parameters) error {
	if t.keyManager.GetRelinearizationKey() == nil {
		return fmt.Errorf("重线性化密钥未准备就绪")
	}
	if t.keyManager.GetGlobalPK() == nil || t.keyManager.GetAggregatedSecretKey() == nil {
		return fmt.Errorf("公钥或私钥未准备就绪")
	}

	encoder := ckks.NewEncoder(params)
	encryptor := ckks.NewEncryptor(params, t.keyManager.GetGlobalPK())
	decryptorAgg := ckks.NewDecryptor(params, t.keyManager.GetAggregatedSecretKey())

	evk := rlwe.NewMemEvaluationKeySet(t.keyManager.GetRelinearizationKey())

	test.TestRelinearizationKey(params, evk, encoder, encryptor, decryptorAgg)
	return nil
}

// TestGaloisKeysOnly 仅测试伽罗瓦密钥
func (t *Tester) TestGaloisKeysOnly(params ckks.Parameters, galEls []uint64) error {
	if len(t.keyManager.GetGaloisKeys()) == 0 {
		return fmt.Errorf("伽罗瓦密钥未准备就绪")
	}
	if t.keyManager.GetAggregatedSecretKey() == nil {
		return fmt.Errorf("聚合私钥未准备就绪")
	}

	evk := rlwe.NewMemEvaluationKeySet(nil, t.keyManager.GetGaloisKeys()...)

	test.TestGaloisKeys(params, t.keyManager.GetExpectedN(), evk, galEls, t.keyManager.GetAggregatedSecretKey())
	return nil
}
