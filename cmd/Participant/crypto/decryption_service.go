package crypto

import (
	"MPHEDev/cmd/Participant/types"
	"MPHEDev/cmd/Participant/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// DecryptionService 解密服务
type DecryptionService struct {
	keyManager *KeyManager
	client     *types.HTTPClient
}

// NewDecryptionService 创建新的解密服务
func NewDecryptionService(keyManager *KeyManager, client *types.HTTPClient) *DecryptionService {
	return &DecryptionService{
		keyManager: keyManager,
		client:     client,
	}
}

// GeneratePartialDecryptShare 生成本地解密份额
func (ds *DecryptionService) GeneratePartialDecryptShare(ciphertext *rlwe.Ciphertext, taskID string) (multiparty.KeySwitchShare, error) {
	params := ds.keyManager.GetParams()

	// 创建解密协议实例
	decryptionProto, err := multiparty.NewKeySwitchProtocol(params, ring.DiscreteGaussian{
		Sigma: 1 << 30,
		Bound: 6 * (1 << 30),
	})
	if err != nil {
		return multiparty.KeySwitchShare{}, err
	}

	level := ciphertext.Level()
	share := decryptionProto.AllocateShare(level)

	// 生成份额（目标密钥为零）
	zeroSk := rlwe.NewSecretKey(params)
	decryptionProto.GenShare(ds.keyManager.GetSecretKey(), zeroSk, ciphertext, &share)

	return share, nil
}

// RequestCollaborativeDecrypt 发起协同解密请求
func (ds *DecryptionService) RequestCollaborativeDecrypt(onlinePeers map[int]string, myID int) error {
	fmt.Println("[协同解密] 自动生成明文并加密...")

	// 生成明文测试用，实际使用时传入待解密的密文
	slots := 8
	values := make([]complex128, slots)
	for i := range values {
		values[i] = complex(rand.Float64()*10, 0)
	}
	fmt.Printf("原始明文: ")
	for i := range values {
		fmt.Printf("%.2f ", real(values[i]))
	}
	fmt.Println()

	// 编码加密
	params := ds.keyManager.GetParams()
	encoder := ckks.NewEncoder(params)
	pt := ckks.NewPlaintext(params, params.MaxLevel())
	if err := encoder.Encode(values, pt); err != nil {
		return fmt.Errorf("编码失败: %v", err)
	}

	encryptor := rlwe.NewEncryptor(params, ds.keyManager.GetPublicKey())
	ct, err := encryptor.EncryptNew(pt)
	if err != nil {
		return fmt.Errorf("加密失败: %v", err)
	}

	// 序列化密文为base64
	ctBytes, err := utils.EncodeShare(ct)
	if err != nil {
		return fmt.Errorf("密文序列化失败: %v", err)
	}
	ctB64 := utils.EncodeToBase64(ctBytes)

	// 自己先算一份解密份额
	myShare, err := ds.GeneratePartialDecryptShare(ct, "task1")
	if err != nil {
		return fmt.Errorf("本地解密份额生成失败: %v", err)
	}

	// 向所有在线Peers（不包括自己）并发请求解密份额
	type peerResp struct {
		PeerID int
		Share  multiparty.KeySwitchShare
		Err    error
	}
	results := make(chan peerResp, len(onlinePeers)-1)

	fmt.Printf("向 %d 个在线参与方请求解密份额...\n", len(onlinePeers)-1)
	for peerID, peerURL := range onlinePeers {
		if peerID == myID {
			continue // 跳过自己
		}
		go func(peerID int, peerURL string) {
			// 构造请求体
			reqBody, _ := json.Marshal(map[string]interface{}{
				"task_id":    "task1",
				"ciphertext": ctB64, // 传递base64字符串
			})
			resp, err := ds.client.Client.Post(peerURL+"/partial_decrypt", "application/json", bytes.NewReader(reqBody))
			if err != nil {
				results <- peerResp{PeerID: peerID, Err: err}
				return
			}
			defer resp.Body.Close()
			var respData struct {
				Share multiparty.KeySwitchShare `json:"share"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
				results <- peerResp{PeerID: peerID, Err: err}
				return
			}
			results <- peerResp{PeerID: peerID, Share: respData.Share}
		}(peerID, peerURL)
	}

	// 收集所有份额
	shares := []multiparty.KeySwitchShare{myShare}
	successCount := 1 // 包括自己的份额
	for i := 0; i < len(onlinePeers)-1; i++ {
		res := <-results
		if res.Err != nil {
			fmt.Printf("[警告] 获取参与方 %d 份额失败: %v\n", res.PeerID, res.Err)
			continue
		}
		shares = append(shares, res.Share)
		successCount++
	}

	fmt.Printf("成功收集 %d 个解密份额 (包括本地份额)\n", successCount)

	// 聚合份额并解密
	ptOut, err := ds.FinalizeCollaborativeDecryption(ct, shares)
	if err != nil {
		return fmt.Errorf("聚合解密失败: %v", err)
	}
	decoded := make([]complex128, slots)
	if err := encoder.Decode(ptOut, decoded); err != nil {
		return fmt.Errorf("解码失败: %v", err)
	}
	fmt.Printf("解密结果: ")
	for i := range decoded {
		fmt.Printf("%.2f ", real(decoded[i]))
	}
	fmt.Println()
	return nil
}

// FinalizeCollaborativeDecryption 聚合份额并输出明文
func (ds *DecryptionService) FinalizeCollaborativeDecryption(ct *rlwe.Ciphertext, shares []multiparty.KeySwitchShare) (*rlwe.Plaintext, error) {
	if ct == nil || len(shares) == 0 {
		return nil, fmt.Errorf("无效输入: 密文为空或无份额")
	}

	params := ds.keyManager.GetParams()
	proto, err := multiparty.NewKeySwitchProtocol(params, ring.DiscreteGaussian{
		Sigma: 1 << 30,
		Bound: 6 * (1 << 30),
	})
	if err != nil {
		return nil, err
	}

	level := ct.Level()
	agg := proto.AllocateShare(level)
	if err := proto.AggregateShares(shares[0], agg, &agg); err != nil {
		return nil, err
	}
	for i := 1; i < len(shares); i++ {
		if err := proto.AggregateShares(shares[i], agg, &agg); err != nil {
			return nil, err
		}
	}

	resultCT := rlwe.NewCiphertext(params, 1, level)
	*resultCT.MetaData = *ct.MetaData
	proto.KeySwitch(ct, agg, resultCT)

	pt := ckks.NewPlaintext(params, level)
	pt.Value.CopyLvl(level, resultCT.Value[0])
	pt.Scale = resultCT.Scale
	pt.IsNTT = resultCT.IsNTT

	return pt, nil
}
