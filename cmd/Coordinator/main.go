package main

import (
	"MPHEDev/pkg/setup"
	test "MPHEDev/pkg/testFunc"
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

type ParticipantInfo struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}
type PublicKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	ShareData     string `json:"share_data"` // base64编码
}
type SecretKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	ShareData     string `json:"share_data"` // base64编码
}

var (
	participants    = make(map[int]*ParticipantInfo)
	nextID          = 1
	params          ckks.Parameters
	paramsLiteral   ckks.ParametersLiteral
	globalCRP       multiparty.PublicKeyGenCRP
	crpBytes        string                 // base64编码的CRP
	publicKeyShares = make(map[int][]byte) // 记录每个参与方的公钥份额
	secretKeyShares = make(map[int][]byte) // 记录每个参与方的私钥
	mu              sync.Mutex
	expectedN       = 0
	globalPK        *rlwe.PublicKey
	skAgg           *rlwe.SecretKey
)

func main() {
	fmt.Print("请输入参与方数量: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	n, err := strconv.Atoi(line)
	if err != nil || n <= 0 {
		fmt.Println("输入有误，使用默认值3")
		expectedN = 3
	} else {
		expectedN = n
	}

	params, err = setup.InitParameters()
	if err != nil {
		panic(err)
	}
	paramsLiteral = params.ParametersLiteral()

	proto := multiparty.NewPublicKeyGenProtocol(params)
	crs, err := sampling.NewPRNG()
	if err != nil {
		panic(err)
	}
	crp := proto.SampleCRP(crs)
	globalCRP = crp
	crpRaw, err := encodeShare(crp)
	if err != nil {
		panic(err)
	}
	crpBytes = base64.StdEncoding.EncodeToString(crpRaw)

	r := gin.Default()
	r.POST("/register", registerHandler)
	r.GET("/params/ckks", getCKKSParamsHandler)
	r.POST("/keys/public", postPublicKeyHandler)
	r.POST("/keys/secret", postSecretKeyHandler)
	r.GET("/participants", getParticipantsHandler)
	r.GET("/setup/status", getSetupStatusHandler)
	r.Run(":8080")
}

func registerHandler(c *gin.Context) {
	mu.Lock()
	id := nextID
	nextID++
	participants[id] = &ParticipantInfo{ID: id, Status: "registered"}
	mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"participant_id": id})
}

func getCKKSParamsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"params": paramsLiteral,
		"crp":    crpBytes,
	})
}

func postPublicKeyHandler(c *gin.Context) {
	var req PublicKeyShare
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.ShareData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64"})
		return
	}
	mu.Lock()
	publicKeyShares[req.ParticipantID] = data
	if p, ok := participants[req.ParticipantID]; ok {
		p.Status = "uploaded_key"
	}
	// 检查是否收到所有份额
	if len(publicKeyShares) == expectedN && globalPK == nil {
		fmt.Println("收到所有公钥份额，开始聚合...")
		go aggregatePublicKey() // 异步聚合
	}
	mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"status": "received"})
}

func postSecretKeyHandler(c *gin.Context) {
	var req SecretKeyShare
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.ShareData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64"})
		return
	}
	mu.Lock()
	secretKeyShares[req.ParticipantID] = data
	if p, ok := participants[req.ParticipantID]; ok {
		p.Status = "uploaded_secret"
	}
	// 检查是否收到所有私钥
	if len(secretKeyShares) == expectedN && skAgg == nil {
		fmt.Println("收到所有私钥，开始聚合...")
		go aggregateSecretKey() // 异步聚合
	}
	mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"status": "received"})
}

func aggregatePublicKey() {
	mu.Lock()
	defer mu.Unlock()
	var aggShare multiparty.PublicKeyGenShare
	first := true
	proto := multiparty.NewPublicKeyGenProtocol(params)
	// 这里直接用 globalCRP
	for _, data := range publicKeyShares {
		var share multiparty.PublicKeyGenShare
		if err := decodeShare(data, &share); err != nil {
			fmt.Println("解码公钥份额失败:", err)
			return
		}
		if first {
			aggShare = share
			first = false
		} else {
			proto.AggregateShares(aggShare, share, &aggShare)
		}
	}
	pk := rlwe.NewPublicKey(params)
	proto.GenPublicKey(aggShare, globalCRP, pk)
	globalPK = pk
	fmt.Println("全局公钥聚合完成！")
	tryTestPublicKey()
}

func aggregateSecretKey() {
	mu.Lock()
	defer mu.Unlock()
	var sks []*rlwe.SecretKey
	for _, data := range secretKeyShares {
		var sk rlwe.SecretKey
		if err := decodeShare(data, &sk); err != nil {
			fmt.Println("解码私钥失败:", err)
			return
		}
		sks = append(sks, &sk)
	}
	skAgg = generateAggregatedSecretKey(params, sks)
	fmt.Println("聚合私钥生成完成！")
	tryTestPublicKey()
}

func tryTestPublicKey() {
	if globalPK != nil && skAgg != nil {
		fmt.Println("\n==== 自动测试协同公钥功能 ====")
		encoder := ckks.NewEncoder(params)
		encryptor := ckks.NewEncryptor(params, globalPK)
		decryptorAgg := ckks.NewDecryptor(params, skAgg)
		test.TestPublicKey(params, encoder, encryptor, decryptorAgg)
	}
}

// 直接复制 setup.GenerateAggregatedSecretKey 的实现
func generateAggregatedSecretKey(params ckks.Parameters, sks []*rlwe.SecretKey) *rlwe.SecretKey {
	skAgg := rlwe.NewSecretKey(params)
	for _, sk := range sks {
		params.RingQP().Add(skAgg.Value, sk.Value, skAgg.Value)
	}
	return skAgg
}

func encodeShare(share interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(share); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeShare(data []byte, share interface{}) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(share)
}

func getParticipantsHandler(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()
	list := make([]*ParticipantInfo, 0, len(participants))
	for _, p := range participants {
		list = append(list, p)
	}
	c.JSON(http.StatusOK, list)
}

func getSetupStatusHandler(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()
	status := gin.H{
		"received_shares":  len(publicKeyShares),
		"received_secrets": len(secretKeyShares),
		"total":            expectedN,
		"global_pk_ready":  globalPK != nil,
		"sk_agg_ready":     skAgg != nil,
	}
	c.JSON(http.StatusOK, status)
}
