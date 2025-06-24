package services

import (
	"MPHEDev/cmd/Coordinator/utils"
	"MPHEDev/pkg/setup"
	test "MPHEDev/pkg/testFunc"
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tuneinsight/lattigo/v6/circuits/ckks/bootstrapping"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	lattigoUtils "github.com/tuneinsight/lattigo/v6/utils"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

type Coordinator struct {
	participants    map[int]*utils.ParticipantInfo
	nextID          int
	params          ckks.Parameters
	paramsLiteral   ckks.ParametersLiteral
	globalCRP       multiparty.PublicKeyGenCRP
	crpBytes        string
	publicKeyShares map[int][]byte
	secretKeyShares map[int][]byte
	expectedN       int
	globalPK        *rlwe.PublicKey
	skAgg           *rlwe.SecretKey
	mu              sync.Mutex

	// 伽罗瓦密钥相关 - 使用正确的协议和类型
	galEls          []uint64
	galoisCRPs      map[uint64]multiparty.GaloisKeyGenCRP
	galoisCRPsBytes map[uint64]string         // base64编码的CRPs
	galoisKeyShares map[uint64]map[int][]byte // galEl -> participantID -> share
	galoisKeys      []*rlwe.GaloisKey
	galoisProto     multiparty.GaloisKeyGenProtocol

	// 重线性化密钥相关
	rlkCRP              multiparty.RelinearizationKeyGenCRP
	rlkCRPBytes         string
	rlkShare1Map        map[int][]byte                         // participantID -> share1
	rlkShare2Map        map[int][]byte                         // participantID -> share2
	rlkShare1Aggregated *multiparty.RelinearizationKeyGenShare // 聚合后的第一轮份额
	rlk                 *rlwe.RelinearizationKey
	rlkProto            multiparty.RelinearizationKeyGenProtocol
	rlkRound            int // 当前轮次：1或2
}

func NewCoordinator(expectedN int) (*Coordinator, error) {
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

	return &Coordinator{
		participants:        make(map[int]*utils.ParticipantInfo),
		nextID:              1,
		params:              params,
		paramsLiteral:       params.ParametersLiteral(),
		globalCRP:           crp,
		crpBytes:            utils.EncodeToBase64(crpRaw),
		publicKeyShares:     make(map[int][]byte),
		secretKeyShares:     make(map[int][]byte),
		expectedN:           expectedN,
		galEls:              galEls,
		galoisCRPs:          galoisCRPs,
		galoisCRPsBytes:     galoisCRPsBytes,
		galoisKeyShares:     make(map[uint64]map[int][]byte),
		galoisKeys:          make([]*rlwe.GaloisKey, 0),
		galoisProto:         galoisProto,
		rlkCRP:              rlkCRP,
		rlkCRPBytes:         utils.EncodeToBase64(rlkCRPRaw),
		rlkShare1Map:        make(map[int][]byte),
		rlkShare2Map:        make(map[int][]byte),
		rlkShare1Aggregated: nil,
		rlk:                 nil,
		rlkProto:            rlkProto,
		rlkRound:            1,
	}, nil
}

func (c *Coordinator) RegisterParticipant() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.nextID
	c.nextID++
	c.participants[id] = &utils.ParticipantInfo{ID: id, Status: "registered"}
	return id
}

func (c *Coordinator) GetParams() (ckks.ParametersLiteral, string, []uint64, map[uint64]string, string) {
	return c.paramsLiteral, c.crpBytes, c.galEls, c.galoisCRPsBytes, c.rlkCRPBytes
}

func (c *Coordinator) GetRelinearizationKeyRound1Aggregated() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.rlkShare1Map) != c.expectedN {
		return "", fmt.Errorf("第一轮份额尚未完全聚合")
	}

	if c.rlkShare1Aggregated == nil {
		return "", fmt.Errorf("第一轮份额聚合结果尚未准备好")
	}

	shareBytes, err := utils.EncodeShare(*c.rlkShare1Aggregated)
	if err != nil {
		return "", err
	}
	return utils.EncodeToBase64(shareBytes), nil
}

func (c *Coordinator) AddPublicKeyShare(participantID int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.publicKeyShares[participantID] = data
	if p, ok := c.participants[participantID]; ok {
		p.Status = "uploaded_key"
	}
	// 检查是否收到所有份额
	if len(c.publicKeyShares) == c.expectedN && c.globalPK == nil {
		fmt.Println("收到所有公钥份额，开始聚合...")
		go c.aggregatePublicKey() // 异步聚合
	}
	return nil
}

func (c *Coordinator) AddSecretKey(participantID int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.secretKeyShares[participantID] = data
	if p, ok := c.participants[participantID]; ok {
		p.Status = "uploaded_secret"
	}
	// 检查是否收到所有私钥
	if len(c.secretKeyShares) == c.expectedN && c.skAgg == nil {
		fmt.Println("收到所有私钥，开始聚合...")
		go c.aggregateSecretKey() // 异步聚合
	}
	return nil
}

func (c *Coordinator) AddGaloisKeyShare(participantID int, galEl uint64, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 初始化该galEl的map
	if c.galoisKeyShares[galEl] == nil {
		c.galoisKeyShares[galEl] = make(map[int][]byte)
	}

	c.galoisKeyShares[galEl][participantID] = data

	// 检查是否收到该galEl的所有份额
	if len(c.galoisKeyShares[galEl]) == c.expectedN {
		go c.aggregateGaloisKey(galEl) // 异步聚合
	}

	return nil
}

func (c *Coordinator) AddRelinearizationKeyShare(participantID int, round int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if round == 1 {
		c.rlkShare1Map[participantID] = data
		// 检查是否收到第一轮的所有份额
		if len(c.rlkShare1Map) == c.expectedN {
			fmt.Println("收到所有重线性化密钥第一轮份额，开始聚合...")
			go c.aggregateRelinearizationKeyRound1()
		}
	} else if round == 2 {
		c.rlkShare2Map[participantID] = data
		// 检查是否收到第二轮的所有份额
		if len(c.rlkShare2Map) == c.expectedN {
			fmt.Println("收到所有重线性化密钥第二轮份额，开始聚合...")
			go c.aggregateRelinearizationKeyRound2()
		}
	}

	return nil
}

func (c *Coordinator) GetParticipants() []*utils.ParticipantInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	list := make([]*utils.ParticipantInfo, 0, len(c.participants))
	for _, p := range c.participants {
		list = append(list, p)
	}
	return list
}

func (c *Coordinator) GetStatus() gin.H {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 计算已完成的伽罗瓦密钥数量
	completedGaloisKeys := 0
	for galEl := range c.galoisKeyShares {
		if len(c.galoisKeyShares[galEl]) == c.expectedN {
			completedGaloisKeys++
		}
	}

	return gin.H{
		"received_shares":       len(c.publicKeyShares),
		"received_secrets":      len(c.secretKeyShares),
		"total":                 c.expectedN,
		"global_pk_ready":       c.globalPK != nil,
		"sk_agg_ready":          c.skAgg != nil,
		"galois_keys_ready":     len(c.galoisKeys),
		"total_galois_keys":     len(c.galEls),
		"completed_galois_keys": completedGaloisKeys,
		"rlk_round1_ready":      c.rlkShare1Aggregated != nil,
		"rlk_round2_ready":      len(c.rlkShare2Map) == c.expectedN,
		"rlk_ready":             c.rlk != nil,
	}
}

// 调用key_aggregation.go中的聚合方法
func (c *Coordinator) aggregatePublicKey() {
	if err := c.AggregatePublicKey(); err != nil {
		fmt.Println("公钥聚合失败:", err)
		return
	}
	fmt.Println("全局公钥聚合完成！")
	c.tryTestKeys()
}

func (c *Coordinator) aggregateSecretKey() {
	if err := c.AggregateSecretKey(); err != nil {
		fmt.Println("私钥聚合失败:", err)
		return
	}
	fmt.Println("聚合私钥生成完成！")
	c.tryTestKeys()
}

func (c *Coordinator) aggregateGaloisKey(galEl uint64) {
	if err := c.AggregateGaloisKey(galEl); err != nil {
		fmt.Printf("伽罗瓦密钥 galEl %d 聚合失败: %v\n", galEl, err)
		return
	}

	// 检查是否所有伽罗瓦密钥都已完成
	completedCount := len(c.galoisKeys)
	totalCount := len(c.galEls)
	//后期需要在前端则将这行重新显示
	//fmt.Printf("伽罗瓦密钥进度: %d/%d\n", completedCount, totalCount)

	// 如果所有伽罗瓦密钥都完成，尝试运行测试
	if completedCount == totalCount {
		fmt.Println("所有伽罗瓦密钥聚合完成！")
		c.tryTestKeys()
	}
}

func (c *Coordinator) tryTestKeys() {
	fmt.Printf("尝试运行测试 - 公钥就绪: %v, 私钥就绪: %v, 伽罗瓦密钥: %d/%d, 重线性化密钥: %v\n",
		c.globalPK != nil, c.skAgg != nil, len(c.galoisKeys), len(c.galEls), c.rlk != nil)

	if c.globalPK != nil && c.skAgg != nil && len(c.galoisKeys) == len(c.galEls) && c.rlk != nil {
		fmt.Println("\n==== 自动测试协同密钥功能 ====")
		encoder := ckks.NewEncoder(c.params)
		encryptor := ckks.NewEncryptor(c.params, c.globalPK)
		decryptorAgg := ckks.NewDecryptor(c.params, c.skAgg)

		// 测试公钥功能
		test.TestPublicKey(c.params, encoder, encryptor, decryptorAgg)

		// 测试重线性化密钥功能
		evk := rlwe.NewMemEvaluationKeySet(c.rlk, c.galoisKeys...)
		test.TestRelinearizationKey(c.params, evk, encoder, encryptor, decryptorAgg)

		// 测试伽罗瓦密钥功能
		test.TestGaloisKeys(c.params, c.expectedN, evk, c.galEls, c.skAgg)
	} else {
		fmt.Println("测试条件未满足，跳过测试")
	}
}

func (c *Coordinator) aggregateRelinearizationKeyRound1() {
	if err := c.AggregateRelinearizationKeyRound1(); err != nil {
		fmt.Println("重线性化密钥第一轮聚合失败:", err)
		return
	}
	fmt.Println("重线性化密钥第一轮聚合完成！")
}

func (c *Coordinator) aggregateRelinearizationKeyRound2() {
	if err := c.AggregateRelinearizationKeyRound2(); err != nil {
		fmt.Println("重线性化密钥第二轮聚合失败:", err)
		return
	}
	fmt.Println("重线性化密钥第二轮聚合完成！")
	c.tryTestKeys()
}
