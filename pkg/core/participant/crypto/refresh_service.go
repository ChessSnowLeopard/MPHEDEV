package crypto

import (
	"MPHEDev/pkg/core/participant/types"
	"MPHEDev/pkg/core/participant/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/multiparty/mpckks"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

// RefreshService 刷新服务
type RefreshService struct {
	keyManager    *KeyManager
	client        *types.HTTPClient
	params        ckks.Parameters
	commonCRSSeed []byte // 统一的CRS种子
}

// NewRefreshService 创建新的刷新服务
func NewRefreshService(keyManager *KeyManager, client *types.HTTPClient) *RefreshService {
	return &RefreshService{
		keyManager: keyManager,
		client:     client,
		params:     keyManager.GetParams(),
	}
}

// UpdateParams 更新参数
func (rs *RefreshService) UpdateParams(params ckks.Parameters) {
	rs.params = params
}

// SetRefreshCRS 设置刷新CRS（从Coordinator获取）
// SetCommonCRSSeed 设置统一的CRS种子
func (rs *RefreshService) SetCommonCRSSeed(seed []byte) {
	rs.commonCRSSeed = seed
}

// GenerateRefreshShare 生成本地刷新份额
func (rs *RefreshService) GenerateRefreshShare(ciphertext *rlwe.Ciphertext, taskID string) (multiparty.RefreshShare, error) {
	refreshNoise := ring.DiscreteGaussian{
		Sigma: 6.36,
		Bound: 128,
	}
	refreshProto, err := mpckks.NewRefreshProtocol(rs.params, 128, refreshNoise)
	if err != nil {
		return multiparty.RefreshShare{}, err
	}

	// 从统一CRS种子生成PRNG
	crs, err := sampling.NewKeyedPRNG(rs.commonCRSSeed)
	if err != nil {
		return multiparty.RefreshShare{}, fmt.Errorf("生成CRS失败: %v", err)
	}
	// 用统一CRS生成CRP
	refreshCRP := refreshProto.SampleCRP(rs.params.MaxLevel(), crs)

	level := ciphertext.Level()
	maxLevel := rs.params.MaxLevel()
	share := refreshProto.AllocateShare(level, maxLevel)

	if err := refreshProto.GenShare(rs.keyManager.GetSecretKey(), 128, ciphertext, refreshCRP, &share); err != nil {
		return multiparty.RefreshShare{}, err
	}

	return share, nil
}

// RequestCollaborativeRefresh 发起协同刷新请求
func (rs *RefreshService) RequestCollaborativeRefresh(onlinePeers map[int]string, myID int) error {
	// 检查参数是否已设置
	if rs.params.LogN() == 0 {
		return fmt.Errorf("CKKS参数未设置，请先完成密钥生成")
	}

	fmt.Println("[协同刷新测试] 自动生成明文并加密...")

	//生成明文 仅用于测试环境
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
	encoder := ckks.NewEncoder(rs.params)
	pt := ckks.NewPlaintext(rs.params, rs.params.MaxLevel())
	if err := encoder.Encode(values, pt); err != nil {
		return fmt.Errorf("编码失败: %v", err)
	}

	encryptor := rlwe.NewEncryptor(rs.params, rs.keyManager.GetPublicKey())
	ct, err := encryptor.EncryptNew(pt)
	if err != nil {
		return fmt.Errorf("加密失败: %v", err)
	}

	// 消耗深度：进行真实的乘法运算来降低密文级别
	fmt.Printf("原始密文: Level=%d, Scale=2^%.2f\n", ct.Level(), ct.Scale.Log2())

	// 进行同态乘法运算消耗深度（模拟真实应用场景）
	evaluator := ckks.NewEvaluator(rs.params, nil)

	// 创建一些随机明文进行乘法运算
	for i := 0; i < 3; i++ {
		if ct.Level() > 0 {
			// 生成随机明文
			randomValues := make([]complex128, slots)
			for j := range randomValues {
				randomValues[j] = complex(rand.Float64()*2+0.5, 0) // 0.5-2.5之间的随机数
			}
			randomPt := ckks.NewPlaintext(rs.params, ct.Level())
			if err := encoder.Encode(randomValues, randomPt); err != nil {
				return fmt.Errorf("随机明文编码失败: %v", err)
			}

			// 执行乘法运算
			evaluator.Mul(ct, randomPt, ct)
			fmt.Printf("第%d次乘法后: Level=%d, Scale=2^%.2f\n", i+1, ct.Level(), ct.Scale.Log2())
		}
	}

	fmt.Printf("消耗深度后密文: Level=%d, Scale=2^%.2f\n", ct.Level(), ct.Scale.Log2())

	// 序列化密文为base64
	ctBytes, err := utils.EncodeShare(ct)
	if err != nil {
		return fmt.Errorf("密文序列化失败: %v", err)
	}
	ctB64 := utils.EncodeToBase64(ctBytes)

	// 自己先算一份刷新份额
	myShare, err := rs.GenerateRefreshShare(ct, "refresh_task1")
	if err != nil {
		return fmt.Errorf("本地刷新份额生成失败: %v", err)
	}

	// 向所有在线Peers（不包括自己）并发请求刷新份额
	type peerResp struct {
		PeerID int
		Share  multiparty.RefreshShare
		Err    error
	}
	results := make(chan peerResp, len(onlinePeers)-1)

	fmt.Printf("向 %d 个在线参与方请求刷新份额...\n", len(onlinePeers)-1)
	for peerID, peerURL := range onlinePeers {
		if peerID == myID {
			continue // 跳过自己
		}
		go func(peerID int, peerURL string) {
			// 构造请求体
			reqBody, _ := json.Marshal(types.RefreshRequest{
				TaskID:     "refresh_task1",
				Ciphertext: ctB64,
			})
			resp, err := rs.client.Client.Post(peerURL+"/partial_refresh", "application/json", bytes.NewReader(reqBody))
			if err != nil {
				results <- peerResp{PeerID: peerID, Err: err}
				return
			}
			defer resp.Body.Close()
			var respData types.RefreshShareResponse
			if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
				results <- peerResp{PeerID: peerID, Err: err}
				return
			}

			// 解析份额
			shareBytes, err := utils.DecodeFromBase64(respData.Share)
			if err != nil {
				results <- peerResp{PeerID: peerID, Err: err}
				return
			}
			var share multiparty.RefreshShare
			if err := utils.DecodeShare(shareBytes, &share); err != nil {
				results <- peerResp{PeerID: peerID, Err: err}
				return
			}
			results <- peerResp{PeerID: peerID, Share: share}
		}(peerID, peerURL)
	}

	// 收集所有份额
	shares := []multiparty.RefreshShare{myShare}
	successCount := 1 // 包括自己的份额
	for i := 0; i < len(onlinePeers)-1; i++ {
		res := <-results
		if res.Err != nil {
			fmt.Printf("[警告] 获取参与方 %d 刷新份额失败: %v\n", res.PeerID, res.Err)
			continue
		}
		shares = append(shares, res.Share)
		successCount++
	}

	fmt.Printf("成功收集 %d 个刷新份额 (包括本地份额)\n", successCount)

	// 聚合份额并刷新
	refreshedCT, err := rs.FinalizeCollaborativeRefresh(ct, shares, "refresh_task1")
	if err != nil {
		return fmt.Errorf("聚合刷新失败: %v", err)
	}

	// 显示刷新效果（不输出解密结果）
	fmt.Printf("刷新效果: Level从 %d 提升到 %d, Scale从 2^%.2f 提升到 2^%.2f\n",
		ct.Level(), refreshedCT.Level(), ct.Scale.Log2(), refreshedCT.Scale.Log2())

	return nil
}

// FinalizeCollaborativeRefresh 聚合份额并输出刷新后的密文
func (rs *RefreshService) FinalizeCollaborativeRefresh(ct *rlwe.Ciphertext, shares []multiparty.RefreshShare, taskID string) (*rlwe.Ciphertext, error) {
	if ct == nil || len(shares) == 0 {
		return nil, fmt.Errorf("无效输入: 密文为空或无份额")
	}

	// 创建刷新协议实例
	refreshNoise := ring.DiscreteGaussian{
		Sigma: 6.36,
		Bound: 128,
	}
	refreshProto, err := mpckks.NewRefreshProtocol(rs.params, 128, refreshNoise)
	if err != nil {
		return nil, err
	}

	// 从统一CRS种子生成PRNG
	crs, err := sampling.NewKeyedPRNG(rs.commonCRSSeed)
	if err != nil {
		return nil, fmt.Errorf("生成CRS失败: %v", err)
	}
	// 使用统一CRS生成CRP
	refreshCRP := refreshProto.SampleCRP(rs.params.MaxLevel(), crs)

	// 聚合份额
	level := ct.Level()
	maxLevel := rs.params.MaxLevel()
	agg := refreshProto.AllocateShare(level, maxLevel)
	agg.MetaData = *ct.MetaData

	for _, share := range shares {
		if err := refreshProto.AggregateShares(&share, &agg, &agg); err != nil {
			return nil, err
		}
	}

	// 最终化
	refreshed := ckks.NewCiphertext(rs.params, 1, maxLevel)
	refreshed.Scale = rs.params.DefaultScale()
	if err := refreshProto.Finalize(ct, refreshCRP, agg, refreshed); err != nil {
		return nil, err
	}

	return refreshed, nil
}

// RefreshCiphertextsSync 同步刷新密文集合
func (rs *RefreshService) RefreshCiphertextsSync(
	ciphertexts []*rlwe.Ciphertext,
	onlinePeers map[int]string,
	myID int,
	taskID string,
) ([]*rlwe.Ciphertext, error) {
	if len(ciphertexts) == 0 {
		return nil, fmt.Errorf("密文集合为空")
	}

	if len(onlinePeers) == 0 {
		return nil, fmt.Errorf("在线参与方列表为空")
	}

	fmt.Printf("🔄 开始同步刷新 %d 个密文...\n", len(ciphertexts))

	// 为每个密文生成本地刷新份额
	myShares := make([]multiparty.RefreshShare, len(ciphertexts))
	for i, ct := range ciphertexts {
		share, err := rs.GenerateRefreshShare(ct, fmt.Sprintf("%s_%d", taskID, i))
		if err != nil {
			return nil, fmt.Errorf("生成密文%d的刷新份额失败: %v", i, err)
		}
		myShares[i] = share
	}

	// 向所有在线Peers（不包括自己）并发请求刷新份额
	type peerResp struct {
		PeerID int
		Shares []multiparty.RefreshShare
		Err    error
	}
	results := make(chan peerResp, len(onlinePeers)-1)

	fmt.Printf("向 %d 个在线参与方请求刷新份额...\n", len(onlinePeers)-1)
	for peerID, peerURL := range onlinePeers {
		if peerID == myID {
			continue // 跳过自己
		}
		go func(peerID int, peerURL string) {
			// 为每个密文构造请求
			peerShares := make([]multiparty.RefreshShare, len(ciphertexts))
			for i, ct := range ciphertexts {
				// 序列化密文
				ctBytes, err := utils.EncodeShare(ct)
				if err != nil {
					results <- peerResp{PeerID: peerID, Err: fmt.Errorf("密文%d序列化失败: %v", i, err)}
					return
				}
				ctB64 := utils.EncodeToBase64(ctBytes)

				// 构造请求体
				reqBody, _ := json.Marshal(types.RefreshRequest{
					TaskID:     fmt.Sprintf("%s_%d", taskID, i),
					Ciphertext: ctB64,
				})

				// 发送请求
				resp, err := rs.client.Client.Post(peerURL+"/partial_refresh", "application/json", bytes.NewReader(reqBody))
				if err != nil {
					results <- peerResp{PeerID: peerID, Err: fmt.Errorf("请求密文%d刷新份额失败: %v", i, err)}
					return
				}
				defer resp.Body.Close()

				// 解析响应
				var respData types.RefreshShareResponse
				if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
					results <- peerResp{PeerID: peerID, Err: fmt.Errorf("解析密文%d响应失败: %v", i, err)}
					return
				}

				// 解析份额
				shareBytes, err := utils.DecodeFromBase64(respData.Share)
				if err != nil {
					results <- peerResp{PeerID: peerID, Err: fmt.Errorf("解码密文%d份额失败: %v", i, err)}
					return
				}
				var share multiparty.RefreshShare
				if err := utils.DecodeShare(shareBytes, &share); err != nil {
					results <- peerResp{PeerID: peerID, Err: fmt.Errorf("反序列化密文%d份额失败: %v", i, err)}
					return
				}
				peerShares[i] = share
			}
			results <- peerResp{PeerID: peerID, Shares: peerShares}
		}(peerID, peerURL)
	}

	// 收集所有份额
	allShares := make([][]multiparty.RefreshShare, len(ciphertexts))
	for i := range allShares {
		allShares[i] = []multiparty.RefreshShare{myShares[i]} // 包含自己的份额
	}

	successCount := 1 // 包括自己的份额
	for i := 0; i < len(onlinePeers)-1; i++ {
		res := <-results
		if res.Err != nil {
			fmt.Printf("[警告] 获取参与方 %d 刷新份额失败: %v\n", res.PeerID, res.Err)
			continue
		}
		// 将每个参与方的份额添加到对应的密文份额集合中
		for j, share := range res.Shares {
			if j < len(allShares) {
				allShares[j] = append(allShares[j], share)
			}
		}
		successCount++
	}

	fmt.Printf("成功收集 %d 个参与方的刷新份额\n", successCount)

	// 对每个密文进行刷新
	refreshedCiphertexts := make([]*rlwe.Ciphertext, len(ciphertexts))
	for i, ct := range ciphertexts {
		fmt.Printf("刷新密文 %d/%d...\n", i+1, len(ciphertexts))
		refreshed, err := rs.FinalizeCollaborativeRefresh(ct, allShares[i], fmt.Sprintf("%s_%d", taskID, i))
		if err != nil {
			return nil, fmt.Errorf("刷新密文%d失败: %v", i, err)
		}
		refreshedCiphertexts[i] = refreshed
		fmt.Printf("密文%d刷新完成: Level从 %d 提升到 %d\n", i, ct.Level(), refreshed.Level())
	}

	fmt.Printf("✅ 同步刷新完成，共刷新 %d 个密文\n", len(refreshedCiphertexts))
	return refreshedCiphertexts, nil
}
