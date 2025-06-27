package main

import (
	"MPHEDev/cmd/Participant/services"
	"MPHEDev/cmd/Participant/utils"
	"fmt"
	"strconv"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

func main() {
	coordinatorURL := "http://localhost:8080"

	// 创建参与方实例
	participant := services.NewParticipant()

	// 1. 注册并获取参数
	if err := participant.Register(coordinatorURL); err != nil {
		panic(err)
	}
	fmt.Printf("注册成功，ID: %d\n", participant.ID)

	// 设置参与方ID到客户端
	participant.CoordinatorClient.SetParticipantID(participant.ID)

	// 2. 获取CKKS参数、CRP和伽罗瓦密钥相关参数
	fmt.Println("正在获取参数...")
	params, err := participant.CoordinatorClient.GetParams()
	if err != nil {
		panic(err)
	}
	fmt.Println("参数获取成功，开始解析...")

	// 将 ParamsResponse 转换为 ckks.Parameters
	ckksParams, err := ckks.NewParametersFromLiteral(params.Params)
	if err != nil {
		panic(err)
	}
	fmt.Println("CKKS参数解析成功...")

	participant.KeyManager.SetParams(ckksParams)
	participant.KeyManager.TotalGaloisKeys = len(params.GalEls)
	fmt.Println("参数设置成功...")

	// 设置刷新服务的参数和CRS
	participant.RefreshService.UpdateParams(ckksParams)
	fmt.Println("刷新服务参数更新成功...")

	// 解码 RefreshCRS
	fmt.Println("开始解码 RefreshCRS...")
	refreshCRSBytes, err := utils.DecodeFromBase64(params.RefreshCRS)
	if err != nil {
		panic(err)
	}
	// 从种子重新生成 KeyedPRNG
	refreshCRS, err := sampling.NewKeyedPRNG(refreshCRSBytes)
	if err != nil {
		panic(err)
	}
	participant.RefreshService.SetRefreshCRS(refreshCRS)
	fmt.Println("RefreshCRS 生成成功...")

	fmt.Printf("获取参数成功，伽罗瓦元素数量: %d\n", len(params.GalEls))

	// 3. 生成本地私钥和公钥份额
	fmt.Println("开始生成本地私钥和公钥份额...")
	// 解码 CRP
	fmt.Println("解码 CRP...")
	crpBytes, err := utils.DecodeFromBase64(params.Crp)
	if err != nil {
		panic(err)
	}
	var crp multiparty.PublicKeyGenCRP
	if err := utils.DecodeShare(crpBytes, &crp); err != nil {
		panic(err)
	}
	fmt.Println("CRP 解码成功...")

	// 解码 GaloisCRPs
	fmt.Println("解码 GaloisCRPs...")
	galoisCRPs := make(map[uint64]multiparty.GaloisKeyGenCRP)
	for galElStr, crpStr := range params.GaloisCRPs {
		galEl, err := strconv.ParseUint(galElStr, 10, 64)
		if err != nil {
			panic(fmt.Errorf("无法解析伽罗瓦元素: %s", galElStr))
		}
		crpBytes, err := utils.DecodeFromBase64(crpStr)
		if err != nil {
			panic(err)
		}
		var galoisCRP multiparty.GaloisKeyGenCRP
		if err := utils.DecodeShare(crpBytes, &galoisCRP); err != nil {
			panic(err)
		}
		galoisCRPs[galEl] = galoisCRP
	}
	fmt.Println("GaloisCRPs 解码成功...")

	// 解码 RlkCRP
	fmt.Println("解码 RlkCRP...")
	rlkCRPBytes, err := utils.DecodeFromBase64(params.RlkCRP)
	if err != nil {
		panic(err)
	}
	var rlkCRP multiparty.RelinearizationKeyGenCRP
	if err := utils.DecodeShare(rlkCRPBytes, &rlkCRP); err != nil {
		panic(err)
	}
	fmt.Println("RlkCRP 解码成功...")

	fmt.Println("创建密钥生成器...")
	keyGen := services.NewKeyGenerator(ckksParams, &crp, params.GalEls, galoisCRPs, &rlkCRP)
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

	// 10. 等待所有密钥生成完成
	fmt.Println("等待所有密钥生成完成...")
	for {
		status, err := participant.CoordinatorClient.PollStatus()
		if err != nil {
			fmt.Println("状态查询失败:", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if status.GlobalPKReady && status.SkAggReady && status.RlkReady &&
			status.CompletedGaloisKeys == status.TotalGaloisKeys {
			fmt.Println("所有密钥生成完成！")
			break
		}
		time.Sleep(2 * time.Second)
	}

	// 11. 获取聚合后的密钥
	fmt.Println("获取聚合后的密钥...")
	keys, err := participant.CoordinatorClient.GetAggregatedKeys()
	if err != nil {
		panic(err)
	}

	// 解码并设置公钥
	pubKeyBytes, err := utils.DecodeFromBase64(keys.PubKey)
	if err != nil {
		panic(err)
	}
	var pubKey rlwe.PublicKey
	if err := utils.DecodeShare(pubKeyBytes, &pubKey); err != nil {
		panic(err)
	}
	participant.KeyManager.SetPublicKey(&pubKey)

	// 解码并设置重线性化密钥
	rlkBytes, err := utils.DecodeFromBase64(keys.RelineKey)
	if err != nil {
		panic(err)
	}
	var rlk rlwe.RelinearizationKey
	if err := utils.DecodeShare(rlkBytes, &rlk); err != nil {
		panic(err)
	}
	participant.KeyManager.SetRelinearizationKey(&rlk)

	// 解码并设置伽罗瓦密钥
	galoisKeys := make([]*rlwe.GaloisKey, 0)
	for _, keyStr := range keys.GaloisKeys {
		keyBytes, err := utils.DecodeFromBase64(keyStr)
		if err != nil {
			panic(err)
		}
		var galoisKey rlwe.GaloisKey
		if err := utils.DecodeShare(keyBytes, &galoisKey); err != nil {
			panic(err)
		}
		galoisKeys = append(galoisKeys, &galoisKey)
	}
	participant.KeyManager.SetGaloisKeys(galoisKeys)

	fmt.Println("所有密钥设置完成！")

	// 12. 加密数据集
	fmt.Println("开始加密数据集...")
	if err := participant.EncryptDataset(); err != nil {
		panic(err)
	}
	fmt.Println("数据集加密完成！")

	// 13. 通知准备就绪
	close(participant.ReadyCh)

	// 14. 运行主循环
	participant.RunMainLoop()
}
