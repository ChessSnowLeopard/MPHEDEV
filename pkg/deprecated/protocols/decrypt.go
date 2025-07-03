package protocols

import (
	"MPHEDev/pkg/participant"
	"fmt"
	"sync"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// MultiPartyDecryption 执行多方解密协议
func MultiPartyDecryption(params ckks.Parameters, N int, parties []*participant.Party,
	cloud *participant.Cloud, ciphertexts map[uint64]*rlwe.Ciphertext) map[uint64]*rlwe.Plaintext {
	fmt.Println("开始执行多方解密协议...")

	// 创建零私钥作为目标密钥
	zeroSk := rlwe.NewSecretKey(params)

	// 为每个密文创建通道
	decryptShareChs := make(map[uint64]chan participant.DecryptShareMsg)
	for key := range ciphertexts {
		decryptShareChs[key] = make(chan participant.DecryptShareMsg, N)
	}

	// 保存解密结果
	results := make(map[uint64]*rlwe.Plaintext)
	resultsMutex := &sync.Mutex{}

	var wg sync.WaitGroup

	// 为每个密文启动一个单独的解密流程
	for key, ct := range ciphertexts {
		keyCopy := key
		ctCopy := ct
		channelCopy := decryptShareChs[key]

		wg.Add(1)
		go func() {
			defer wg.Done()

			fmt.Printf("开始解密密文 %d...\n", keyCopy)

			// 为每个参与方生成份额
			for _, p := range parties {
				go runPartialDecrypt(params, participant.DecryptTask{
					Key:        keyCopy,
					Ciphertext: ctCopy,
				}, zeroSk, channelCopy, p)
			}

			// 收集所有参与方的份额
			shareList := make([]multiparty.KeySwitchShare, 0, N)
			for i := 0; i < N; i++ {
				msg := <-channelCopy
				if msg.Key != keyCopy {
					panic(fmt.Sprintf("收到错误的密文份额，期望 %d,实际 %d", keyCopy, msg.Key))
				}
				shareList = append(shareList, msg.Share)
			}

			// 聚合份额并完成解密
			pt := finalizeDecryption(params, ctCopy, shareList)

			// 安全地添加到结果映射
			resultsMutex.Lock()
			results[keyCopy] = pt
			resultsMutex.Unlock()

			fmt.Printf("密文 %d 解密完成\n", keyCopy)
		}()
	}

	// 等待所有解密操作完成
	wg.Wait()

	// 清理通道
	for _, ch := range decryptShareChs {
		close(ch)
	}

	return results
}

// runPartialDecrypt 执行部分解密
func runPartialDecrypt(params ckks.Parameters, task participant.DecryptTask, targetSk *rlwe.SecretKey,
	ch chan<- participant.DecryptShareMsg, p *participant.Party) {
	// 创建解密协议实例
	decryptProto, err := multiparty.NewKeySwitchProtocol(params, ring.DiscreteGaussian{
		Sigma: 1 << 30,
		Bound: 6 * (1 << 30),
	})
	if err != nil {
		panic(err)
	}

	// 分配正确级别的份额
	level := task.Ciphertext.Level()
	share := decryptProto.AllocateShare(level)

	// 生成份额
	decryptProto.GenShare(p.Sk, targetSk, task.Ciphertext, &share)

	// 发送份额到通道
	ch <- participant.DecryptShareMsg{Key: task.Key, Share: share}
}

// finalizeDecryption 最终化解密
func finalizeDecryption(params ckks.Parameters, ct *rlwe.Ciphertext, shares []multiparty.KeySwitchShare) *rlwe.Plaintext {
	if ct == nil || len(shares) == 0 {
		panic("无效输入: 密文为空或无份额")
	}

	// 使用相同的噪声参数创建协议
	proto, err := multiparty.NewKeySwitchProtocol(params, ring.DiscreteGaussian{
		Sigma: 1 << 30,
		Bound: 6 * (1 << 30),
	})
	if err != nil {
		panic(err)
	}

	// 获取正确的级别
	level := ct.Level()

	// 聚合份额
	agg := proto.AllocateShare(level)
	if err := proto.AggregateShares(shares[0], agg, &agg); err != nil {
		panic(fmt.Sprintf("聚合份额错误: %v", err))
	}

	for i := 1; i < len(shares); i++ {
		if err := proto.AggregateShares(shares[i], agg, &agg); err != nil {
			panic(fmt.Sprintf("聚合份额 %d 错误: %v", i, err))
		}
	}

	// 创建临时密文用于密钥切换
	resultCT := rlwe.NewCiphertext(params, 1, level)
	*resultCT.MetaData = *ct.MetaData

	// 执行密钥切换操作
	proto.KeySwitch(ct, agg, resultCT)

	// 提取明文结果
	pt := ckks.NewPlaintext(params, level)
	pt.Value.CopyLvl(level, resultCT.Value[0])
	pt.Scale = resultCT.Scale
	pt.IsNTT = resultCT.IsNTT

	return pt
}
