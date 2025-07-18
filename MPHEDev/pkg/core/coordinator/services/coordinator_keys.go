package services

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// ==================== 密钥管理方法 ====================

// checkAndTestAllKeys 检查所有密钥是否完成，如果完成则进行最终测试
func (c *Coordinator) checkAndTestAllKeys() {
	status := c.GetStatus()

	// 检查所有密钥是否都已完成
	allKeysReady := status["global_pk_ready"].(bool) &&
		status["sk_agg_ready"].(bool) &&
		status["rlk_ready"].(bool) &&
		status["completed_galois_keys"].(int) == status["total_galois_keys"].(int)

	if allKeysReady {
		fmt.Println("\n 所有密钥生成完成！")
		fmt.Println(" 开始最终密钥测试...")

		if err := c.TestAllKeys(); err != nil {
			fmt.Printf(" 最终密钥测试失败: %v\n", err)
		} else {
			fmt.Println(" 所有密钥测试通过！系统准备就绪。")
		}
	}
}

// AddPublicKeyShare 添加公钥份额
func (c *Coordinator) AddPublicKeyShare(participantID int, data []byte) error {
	if err := c.KeyManager.AddPublicKeyShare(participantID, data); err != nil {
		return err
	}

	// 检查是否所有份额都已收集完成，如果是则自动聚合
	publicKeyShares := c.KeyManager.GetPublicKeyShares()
	if len(publicKeyShares) == c.expectedN {
		fmt.Println("\n 开始聚合公钥...")
		globalCRP := c.ParameterManager.GetGlobalCRP()
		if err := c.KeyAggregator.AggregatePublicKey(globalCRP); err != nil {
			return fmt.Errorf("公钥聚合失败: %v", err)
		}

		// 自动测试公钥
		fmt.Println(" 开始测试公钥...")
		if err := c.TestPublicKeyOnly(); err != nil {
			fmt.Printf(" 公钥测试失败: %v\n", err)
		}

		// 检查是否所有密钥都已完成
		c.checkAndTestAllKeys()
	}

	return nil
}

// AddSecretKey 添加私钥
func (c *Coordinator) AddSecretKey(participantID int, data []byte) error {
	if err := c.KeyManager.AddSecretKey(participantID, data); err != nil {
		return err
	}

	// 检查是否所有私钥都已收集完成，如果是则自动聚合
	secretKeyShares := c.KeyManager.GetSecretKeyShares()
	if len(secretKeyShares) == c.expectedN {
		fmt.Println("\n 开始聚合私钥...")
		if err := c.KeyAggregator.AggregateSecretKey(); err != nil {
			return fmt.Errorf("私钥聚合失败: %v", err)
		}

		// 检查是否所有密钥都已完成
		c.checkAndTestAllKeys()
	}

	return nil
}

// AddGaloisKeyShare 添加伽罗瓦密钥份额
func (c *Coordinator) AddGaloisKeyShare(participantID int, galEl uint64, data []byte) error {
	if err := c.KeyManager.AddGaloisKeyShare(participantID, galEl, data); err != nil {
		return err
	}

	// 检查该galEl的所有份额是否都已收集完成，如果是则自动聚合
	galoisKeyShares := c.KeyManager.GetGaloisKeyShares()
	shares := galoisKeyShares[galEl]
	if len(shares) == c.expectedN {
		fmt.Printf("\n 开始聚合伽罗瓦密钥 (galEl: %d)...\n", galEl)
		galoisCRPs := c.ParameterManager.GetGaloisCRPs()
		galoisCRP := galoisCRPs[galEl]
		if err := c.KeyAggregator.AggregateGaloisKey(galEl, galoisCRP); err != nil {
			return fmt.Errorf("伽罗瓦密钥聚合失败 (galEl: %d): %v", galEl, err)
		}

		// 检查是否所有密钥都已完成
		c.checkAndTestAllKeys()
	}

	return nil
}

// AddRelinearizationKeyShare 添加重线性化密钥份额
func (c *Coordinator) AddRelinearizationKeyShare(participantID int, round int, data []byte) error {
	if err := c.KeyManager.AddRelinearizationKeyShare(participantID, round, data); err != nil {
		return err
	}

	if round == 1 {
		// 检查第一轮份额是否都已收集完成，如果是则自动聚合
		rlkShare1Map := c.KeyManager.GetRelinearizationShare1Map()
		fmt.Printf("DEBUG: 参与方 %d 提交第一轮份额，当前进度: %d/%d\n", participantID, len(rlkShare1Map), c.expectedN)

		if len(rlkShare1Map) == c.expectedN {
			fmt.Println("\n 开始聚合重线性化密钥第一轮...")
			if err := c.KeyAggregator.AggregateRelinearizationKeyRound1(); err != nil {
				return fmt.Errorf("重线性化密钥第一轮聚合失败: %v", err)
			}
			fmt.Println(" 重线性化密钥第一轮聚合完成，参与方可以获取聚合结果并提交第二轮份额")

			// 验证聚合结果是否正确设置
			if c.KeyManager.GetRelinearizationShare1Aggregated() != nil {
				fmt.Println("DEBUG: 第一轮聚合结果已正确设置")
			} else {
				fmt.Println("ERROR: 第一轮聚合结果未正确设置")
			}
		} else {
			fmt.Printf(" 重线性化密钥第一轮份额收集进度: %d/%d\n", len(rlkShare1Map), c.expectedN)
		}
	} else if round == 2 {
		// 检查第二轮份额是否都已收集完成，如果是则自动聚合
		rlkShare2Map := c.KeyManager.GetRelinearizationShare2Map()
		fmt.Printf("DEBUG: 参与方 %d 提交第二轮份额，当前进度: %d/%d\n", participantID, len(rlkShare2Map), c.expectedN)

		if len(rlkShare2Map) == c.expectedN {
			fmt.Println("\n 开始聚合重线性化密钥第二轮...")
			if err := c.KeyAggregator.AggregateRelinearizationKeyRound2(); err != nil {
				return fmt.Errorf("重线性化密钥第二轮聚合失败: %v", err)
			}

			// 自动测试重线性化密钥
			fmt.Println(" 开始测试重线性化密钥...")
			if err := c.TestRelinearizationKeyOnly(); err != nil {
				fmt.Printf(" 重线性化密钥测试失败: %v\n", err)
			}

			// 检查是否所有密钥都已完成
			c.checkAndTestAllKeys()
		} else {
			fmt.Printf(" 重线性化密钥第二轮份额收集进度: %d/%d\n", len(rlkShare2Map), c.expectedN)
		}
	}

	return nil
}

// GetRelinearizationKeyRound1Aggregated 获取聚合后的第一轮重线性化密钥份额
func (c *Coordinator) GetRelinearizationKeyRound1Aggregated() (string, error) {
	return c.KeyManager.GetRelinearizationKeyRound1Aggregated()
}

// ==================== 状态管理方法 ====================

// GetStatus 获取设置状态
func (c *Coordinator) GetStatus() gin.H {
	participants := c.ParticipantManager.GetParticipants()
	publicKeyShares := c.KeyManager.GetPublicKeyShares()
	secretKeyShares := c.KeyManager.GetSecretKeyShares()
	galoisKeyShares := c.KeyManager.GetGaloisKeyShares()
	rlkShare2Map := c.KeyManager.GetRelinearizationShare2Map()

	globalPKReady := c.KeyManager.GetGlobalPK() != nil
	skAggReady := c.KeyManager.GetAggregatedSecretKey() != nil
	galoisKeysReady := len(c.KeyManager.GetGaloisKeys())
	totalGaloisKeys := len(c.ParameterManager.GetGalEls())
	completedGaloisKeys := 0

	for galEl := range galoisKeyShares {
		if len(galoisKeyShares[galEl]) == c.expectedN {
			completedGaloisKeys++
		}
	}

	rlkRound1Ready := c.KeyManager.GetRelinearizationShare1Aggregated() != nil
	rlkRound2Ready := len(rlkShare2Map) == c.expectedN
	rlkReady := c.KeyManager.GetRelinearizationKey() != nil

	return gin.H{
		"received_shares":       len(publicKeyShares),
		"received_secrets":      len(secretKeyShares),
		"total":                 len(participants),
		"global_pk_ready":       globalPKReady,
		"sk_agg_ready":          skAggReady,
		"galois_keys_ready":     galoisKeysReady,
		"total_galois_keys":     totalGaloisKeys,
		"completed_galois_keys": completedGaloisKeys,
		"rlk_round1_ready":      rlkRound1Ready,
		"rlk_round2_ready":      rlkRound2Ready,
		"rlk_ready":             rlkReady,
	}
}

// ==================== 密钥测试方法 ====================

// TestAllKeys 测试所有密钥
func (c *Coordinator) TestAllKeys() error {
	params := c.ParameterManager.GetCKKSParams()
	galEls := c.ParameterManager.GetGalEls()
	return c.KeyTester.TestAllKeys(params, galEls)
}

// TestPublicKeyOnly 仅测试公钥
func (c *Coordinator) TestPublicKeyOnly() error {
	params := c.ParameterManager.GetCKKSParams()
	return c.KeyTester.TestPublicKeyOnly(params)
}

// TestRelinearizationKeyOnly 仅测试重线性化密钥
func (c *Coordinator) TestRelinearizationKeyOnly() error {
	params := c.ParameterManager.GetCKKSParams()
	return c.KeyTester.TestRelinearizationKeyOnly(params)
}

// TestGaloisKeysOnly 仅测试伽罗瓦密钥
func (c *Coordinator) TestGaloisKeysOnly() error {
	params := c.ParameterManager.GetCKKSParams()
	galEls := c.ParameterManager.GetGalEls()
	return c.KeyTester.TestGaloisKeysOnly(params, galEls)
}
