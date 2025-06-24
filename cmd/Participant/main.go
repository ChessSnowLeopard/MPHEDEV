package main

import (
	"MPHEDev/cmd/Participant/services"
	"fmt"
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

	// 2. 获取CKKS参数和CRP
	params, crp, err := participant.GetParams(coordinatorURL)
	if err != nil {
		panic(err)
	}
	fmt.Println("CKKS参数:", *params)

	// 3. 生成本地私钥和公钥份额
	keyGen := services.NewKeyGenerator(*params, crp)
	sk, share, err := keyGen.GenerateKeys()
	if err != nil {
		panic(err)
	}

	// 4. 编码并上传私钥
	skB64, err := keyGen.EncodeSecretKey(sk)
	if err != nil {
		panic(err)
	}
	if err := participant.UploadSecretKey(coordinatorURL, skB64); err != nil {
		panic(err)
	}
	fmt.Println("上传私钥成功")

	// 5. 编码并上传公钥份额
	shareB64, err := keyGen.EncodePublicKeyShare(share)
	if err != nil {
		panic(err)
	}
	if err := participant.UploadPublicKeyShare(coordinatorURL, shareB64); err != nil {
		panic(err)
	}
	fmt.Println("上传公钥份额成功")

	// 6. 常驻在线，轮询全局公钥状态
	if err := participant.WaitForCompletion(coordinatorURL); err != nil {
		panic(err)
	}
}
