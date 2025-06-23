package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"MPHEDev/pkg/dataProcess"

	"MPHEDev/pkg/network"
	_ "MPHEDev/pkg/participant"
	"MPHEDev/pkg/protocols"
	"MPHEDev/pkg/setup"
	test "MPHEDev/pkg/testFunc"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

/*
这个main函数是一个多方同态加密分布式操作演示系统的入口点
*/
func main() {
	fmt.Println("======================================================")
	fmt.Println("     多方同态加密分布式操作演示系统")
	fmt.Println("======================================================")
	fmt.Println()

	// 记录开始时间
	startTime := time.Now()

	// 设置参与方数量
	var N int
	fmt.Print("请输入参与方数量: ")
	if _, err := fmt.Scan(&N); err != nil {
		//Scan读一个整数并存到
		fmt.Println("输入有误，使用默认值 3")
		N = 3
	}

	// 初始化系统
	fmt.Println("初始化系统参数和参与方...")
	params, parties, cloud, galEls, crs, err := setup.InitializeSystem(N)
	if err != nil {
		fmt.Printf("系统初始化失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("系统初始化完成，伽罗瓦元素数量: %d\n", len(galEls))

	// 生成聚合私钥（仅用于测试验证）
	skAgg := setup.GenerateAggregatedSecretKey(params, parties)

	// 1. 分布式公钥生成
	fmt.Println("\n1. 执行分布式公钥生成协议")
	protocols.GeneratePublicKey(params, N, parties, cloud)
	pk := <-cloud.PkgDone
	fmt.Println("✓ 公钥生成完成")

	// 初始化编码器和加密器
	encoder := ckks.NewEncoder(params)
	encryptor := ckks.NewEncryptor(params, pk)
	decryptorAgg := ckks.NewDecryptor(params, skAgg)

	// 2. 分布式伽罗瓦密钥生成
	fmt.Println("\n2. 执行分布式伽罗瓦密钥生成协议")
	protocols.GenerateGaloisKeys(params, N, parties, cloud, galEls)

	gks := []*rlwe.GaloisKey{}
	for task := range cloud.GalKeyDone {
		gks = append(gks, task)
	}
	fmt.Printf("✓ %d个伽罗瓦密钥生成完成\n", len(gks))

	// 3. 分布式重线性化密钥生成
	fmt.Println("\n3. 执行分布式重线性化密钥生成协议")
	protocols.GenerateRelinearizationKey(params, N, parties, cloud)
	rlk := <-cloud.RlkDone
	fmt.Println("✓ 重线性化密钥生成完成")

	// 生成评估密钥集
	evk := rlwe.NewMemEvaluationKeySet(rlk, gks...)
	evaluator := ckks.NewEvaluator(params, evk)

	// 4. 测试生成的密钥
	fmt.Println("\n4. 测试生成的密钥功能")
	test.TestPublicKey(params, encoder, encryptor, decryptorAgg)
	test.TestRelinearizationKey(params, evk, encoder, encryptor, decryptorAgg)
	test.TestGaloisKeys(params, N, evk, galEls, skAgg)

	// 5. 测试多方解密
	fmt.Println("\n5. 执行多方解密协议测试")
	test.TestMultiPartyDecryption(params, N, parties, cloud, encoder, encryptor, decryptorAgg)

	// 6. 分布式自举/刷新测试
	fmt.Println("\n6. 执行分布式自举/刷新协议")
	ciphertexts, plaintextsMul := test.TestRefreshOperation(params, encoder, encryptor, evaluator)

	protocols.RefreshCiphertexts(params, N, parties, cloud, ciphertexts, crs)

	fmt.Println("\n自举/刷新结果验证:")
	for result := range cloud.RefreshDone {
		fmt.Printf("密文 %d 刷新完成\n", result.Key)
		fmt.Printf("刷新后 Level=%d, Scale=2^%.2f\n",
			result.Ciphertext.Level(), result.Ciphertext.Scale.Log2())

		// 验证刷新结果
		test.PrintDebugInfo(params, result.Ciphertext, plaintextsMul[result.Key],
			decryptorAgg, encoder)
	}

	//所有分布式协议测试执行完成，接下来将密钥分发到各参与方
	fmt.Println("\n7. 分发密钥到各参与方")
	for _, party := range parties {
		party.Pk = pk
		party.Rlk = rlk
		party.Gks = gks
	}

	// 计算总执行时间
	duration := time.Since(startTime)

	fmt.Println("\n======================================================")
	fmt.Printf("所有分布式协议测试执行完成！\n")
	fmt.Printf("总执行时间: %v\n", duration)

	fmt.Println("\n协议执行总结:")
	fmt.Println("✓ 分布式公钥生成协议")
	fmt.Println("✓ 分布式伽罗瓦密钥生成协议")
	fmt.Println("✓ 分布式重线性化密钥生成协议")
	fmt.Println("✓ 多方解密协议")
	fmt.Println("✓ 分布式自举/刷新协议")
	fmt.Println("✓ 分布式密钥分发")
	fmt.Println("\n所有测试均已通过！")
	fmt.Println("======================================================")

	fmt.Println("\n======================================================")
	fmt.Println("     开始配置分布式神经网络")
	fmt.Println("======================================================")

	// 加载数据集
	trainDataset, testDataset, err := dataProcess.LoadDataset()
	if err != nil {
		log.Fatalf("加载数据集失败: %v", err)
	}

	// 打印数据集信息
	fmt.Printf("训练数据集包含 %d 个样本\n", len(trainDataset.Images))
	fmt.Printf("测试数据集包含 %d 个样本\n", len(testDataset.Images))

	// 配置网络参数
	networkConfig := network.NetworkConfig{
		InputSize:   len(trainDataset.Images[0]), // 输入维度
		OutputSize:  10,                          // 输出类别数
		HiddenSize:  64,                          // 隐藏层节点数
		NumParties:  N,                           // 参与方数量
		EnableDebug: true,                        // 启用调试输出
	}

	// 创建神经网络协调器
	nnCoordinator := network.NewNNCoordinator(
		params,  // 同态加密参数
		pk,      // 多方公钥
		skAgg,   // 聚合私钥（用于验证）
		parties, // 参与方列表
		networkConfig,
	)

	// 验证网络结构
	if err := nnCoordinator.ValidateNetwork(); err != nil {
		log.Fatalf("网络结构验证失败: %v", err)
	}

	// 初始化完成的神经网络各个参与方和用户
	//nnParties := nnCoordinator.GetNNParties()
	//user := nnCoordinator.GetUser()
	inputSize, outputSize, hiddenSize, numParties := nnCoordinator.GetNetworkInfo()

	fmt.Printf("\n网络初始化成功！")
	fmt.Printf("输入维度: %d, 输出维度: %d, 隐藏层大小: %d, 参与方数量: %d\n",
		inputSize, outputSize, hiddenSize, numParties)
}
