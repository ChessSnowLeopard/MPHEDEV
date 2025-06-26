package main

import (
	"MPHEDev/cmd/Participant/services"
	"MPHEDev/cmd/Participant/utils"
	"fmt"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

func main() {
	coordinatorURL := "http://localhost:8080"

	// 创建参与方实例
	participant := services.NewParticipant()

	// 1. 注册
	if err := participant.Register(coordinatorURL); err != nil {
		panic(err)
	}
	fmt.Printf("注册成功，ID: %d\n", participant.ID)

	// 设置参与方ID到客户端
	participant.CoordinatorClient.SetParticipantID(participant.ID)

	// 2. 获取CKKS参数、CRP和伽罗瓦密钥相关参数
	params, crp, galEls, galoisCRPs, rlkCRP, refreshCRS, err := participant.CoordinatorClient.GetParams()
	if err != nil {
		panic(err)
	}
	participant.KeyManager.SetParams(*params)
	participant.KeyManager.TotalGaloisKeys = len(galEls)

	// 设置刷新服务的参数和CRS
	participant.RefreshService.UpdateParams(*params)
	participant.RefreshService.SetRefreshCRS(refreshCRS) // 使用从Coordinator获取的CRS

	fmt.Printf("获取参数成功，伽罗瓦元素数量: %d\n", len(galEls))

	// 3. 生成本地私钥和公钥份额
	keyGen := services.NewKeyGenerator(*params, crp, galEls, galoisCRPs, rlkCRP)
	sk, share, err := keyGen.GenerateKeys()
	if err != nil {
		panic(err)
	}
	participant.KeyManager.SetSecretKey(sk)

	// 4. 编码并上传私钥  该方法仅用于测试环境
	skB64, err := keyGen.EncodeSecretKey(sk)
	if err != nil {
		panic(err)
	}
	if err := participant.CoordinatorClient.UploadSecretKey(skB64); err != nil {
		panic(err)
	}
	fmt.Println("上传私钥成功")

	// 5. 编码并上传公钥份额
	shareB64, err := keyGen.EncodePublicKeyShare(share)
	if err != nil {
		panic(err)
	}
	if err := participant.CoordinatorClient.UploadPublicKeyShare(shareB64); err != nil {
		panic(err)
	}
	fmt.Println("上传公钥份额成功")

	// 6. 生成并上传伽罗瓦密钥份额
	fmt.Println("开始生成伽罗瓦密钥份额...")
	galoisShares, err := keyGen.GenerateGaloisKeyShares()
	if err != nil {
		panic(err)
	}

	for galEl, share := range galoisShares {
		shareB64, err := keyGen.EncodeGaloisKeyShare(share)
		if err != nil {
			panic(err)
		}
		if err := participant.CoordinatorClient.UploadGaloisKeyShare(galEl, shareB64); err != nil {
			panic(err)
		}
	}
	fmt.Printf("✓ 所有 %d 个伽罗瓦密钥份额上传完成\n", len(galoisShares))

	// 7. 生成并上传重线性化密钥第一轮份额
	fmt.Println("开始生成重线性化密钥第一轮份额...")
	if err := keyGen.GenerateRelinearizationKeyRound1(); err != nil {
		panic(err)
	}
	rlkShare1B64, err := keyGen.EncodeRelinearizationKeyShare(1)
	if err != nil {
		panic(err)
	}
	if err := participant.CoordinatorClient.UploadRelinearizationKeyShare(1, rlkShare1B64); err != nil {
		panic(err)
	}
	fmt.Println("上传重线性化密钥第一轮份额成功")

	// 8. 等待第一轮聚合完成，然后获取聚合结果
	fmt.Println("等待第一轮聚合完成...")
	for {
		status, err := participant.CoordinatorClient.PollStatus()
		if err != nil {
			fmt.Println("状态查询失败:", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if status.RlkRound1Ready {
			fmt.Println("第一轮聚合完成，开始第二轮...")
			break
		}
		time.Sleep(2 * time.Second)
	}

	// 9. 获取聚合后的第一轮份额，生成第二轮份额
	aggregatedShare1, err := participant.CoordinatorClient.GetRelinearizationKeyRound1Aggregated()
	if err != nil {
		panic(err)
	}
	if err := keyGen.GenerateRelinearizationKeyRound2(aggregatedShare1); err != nil {
		panic(err)
	}
	rlkShare2B64, err := keyGen.EncodeRelinearizationKeyShare(2)
	if err != nil {
		panic(err)
	}
	if err := participant.CoordinatorClient.UploadRelinearizationKeyShare(2, rlkShare2B64); err != nil {
		panic(err)
	}
	fmt.Println("上传重线性化密钥第二轮份额成功")

	// 10. 常驻在线，轮询全局状态
	if err := participant.CoordinatorClient.WaitForCompletion(); err != nil {
		panic(err)
	}

	// 11. 获取聚合后的密钥
	fmt.Println("获取聚合后的密钥...")
	keys, err := participant.CoordinatorClient.GetAggregatedKeys()
	if err != nil {
		panic(err)
	}

	// 解析并设置公钥
	pubKeyBytes, err := utils.DecodeFromBase64(keys.PubKey)
	if err != nil {
		panic(err)
	}
	var pubKey rlwe.PublicKey
	if err := utils.DecodeShare(pubKeyBytes, &pubKey); err != nil {
		panic(err)
	}
	participant.KeyManager.SetPublicKey(&pubKey)

	// 解析并设置重线性化密钥
	relineKeyBytes, err := utils.DecodeFromBase64(keys.RelineKey)
	if err != nil {
		panic(err)
	}
	var relineKey rlwe.RelinearizationKey
	if err := utils.DecodeShare(relineKeyBytes, &relineKey); err != nil {
		panic(err)
	}
	participant.KeyManager.SetRelinearizationKey(&relineKey)

	fmt.Println("密钥获取完成！")

	// 12. 通知主循环密钥分发完成
	close(participant.ReadyCh)

	// === 启动主循环 ===
	participant.RunMainLoop()
}
