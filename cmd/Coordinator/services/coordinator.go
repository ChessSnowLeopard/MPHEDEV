package services

import (
	"MPHEDev/cmd/Coordinator/utils"
	"MPHEDev/pkg/setup"
	test "MPHEDev/pkg/testFunc"
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
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
}

func NewCoordinator(expectedN int) (*Coordinator, error) {
	params, err := setup.InitParameters()
	if err != nil {
		return nil, err
	}

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

	return &Coordinator{
		participants:    make(map[int]*utils.ParticipantInfo),
		nextID:          1,
		params:          params,
		paramsLiteral:   params.ParametersLiteral(),
		globalCRP:       crp,
		crpBytes:        utils.EncodeToBase64(crpRaw),
		publicKeyShares: make(map[int][]byte),
		secretKeyShares: make(map[int][]byte),
		expectedN:       expectedN,
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

func (c *Coordinator) GetParams() (ckks.ParametersLiteral, string) {
	return c.paramsLiteral, c.crpBytes
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
	return gin.H{
		"received_shares":  len(c.publicKeyShares),
		"received_secrets": len(c.secretKeyShares),
		"total":            c.expectedN,
		"global_pk_ready":  c.globalPK != nil,
		"sk_agg_ready":     c.skAgg != nil,
	}
}

// 调用key_aggregation.go中的聚合方法
func (c *Coordinator) aggregatePublicKey() {
	if err := c.AggregatePublicKey(); err != nil {
		fmt.Println("公钥聚合失败:", err)
		return
	}
	fmt.Println("全局公钥聚合完成！")
	c.tryTestPublicKey()
}

func (c *Coordinator) aggregateSecretKey() {
	if err := c.AggregateSecretKey(); err != nil {
		fmt.Println("私钥聚合失败:", err)
		return
	}
	fmt.Println("聚合私钥生成完成！")
	c.tryTestPublicKey()
}

func (c *Coordinator) tryTestPublicKey() {
	if c.globalPK != nil && c.skAgg != nil {
		fmt.Println("\n==== 自动测试协同公钥功能 ====")
		encoder := ckks.NewEncoder(c.params)
		encryptor := ckks.NewEncryptor(c.params, c.globalPK)
		decryptorAgg := ckks.NewDecryptor(c.params, c.skAgg)
		test.TestPublicKey(c.params, encoder, encryptor, decryptorAgg)
	}
}
