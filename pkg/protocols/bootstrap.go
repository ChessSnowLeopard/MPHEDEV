package protocols

import (
	"MPHEDev/pkg/participant"
	"fmt"
	"sync"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/multiparty/mpckks"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

// RefreshCiphertexts 执行分布式自举/刷新协议
func RefreshCiphertexts(params ckks.Parameters, N int, parties []*participant.Party,
	cloud *participant.Cloud, ciphertexts map[uint64]*rlwe.Ciphertext,
	crs *sampling.KeyedPRNG) {
	fmt.Printf("开始刷新%d个密文...\n", len(ciphertexts))

	// 初始化刷新协议
	refreshNoise := ring.DiscreteGaussian{
		Sigma: 6.36,
		Bound: 128,
	}
	refreshProto, err := mpckks.NewRefreshProtocol(params, 128, refreshNoise)
	if err != nil {
		panic(err)
	}

	cloud.ToRefreshCts = ciphertexts
	cloud.RefreshProto = refreshProto

	// 为参与方设置刷新协议
	for _, p := range parties {
		p.RefreshProtocol = refreshProto.ShallowCopy()
		p.GenTaskQueue = make(chan interface{}, 1)
	}

	// 为每个密文生成CRP和通道
	maxLevel := params.MaxLevel()
	cloud.RefreshCRPs = make(map[uint64]multiparty.KeySwitchCRP)
	cloud.RefShareChs = make(map[uint64]chan multiparty.RefreshShare)
	for key := range ciphertexts {
		cloud.RefreshCRPs[key] = refreshProto.SampleCRP(maxLevel, crs)
		cloud.RefShareChs[key] = make(chan multiparty.RefreshShare, N)
	}
	cloud.RefreshDone = make(chan participant.RefreshDone, len(ciphertexts))

	// 启动刷新协程
	var wgRefresh sync.WaitGroup
	refreshTask := participant.RefreshTask{
		Ciphertexts: ciphertexts,
		Wg:          &wgRefresh,
	}

	for _, p := range parties {
		p.GenTaskQueue <- refreshTask
	}

	for _, p := range parties {
		wgRefresh.Add(1)
		go runPartyBootstrap(params, N, parties, cloud, p)
	}

	// 启动云端协程
	go runCloudBootstrap(params, N, cloud)
	wgRefresh.Wait()

	// 清理
	for key := range ciphertexts {
		close(cloud.RefShareChs[key])
	}
	for _, p := range parties {
		close(p.GenTaskQueue)
	}

	fmt.Println("密文刷新协议完成")
}

func runCloudBootstrap(params ckks.Parameters, t int, c *participant.Cloud) {
	var wg sync.WaitGroup

	for key, ct := range c.ToRefreshCts {
		wg.Add(1)

		keyCopy := key
		ctCopy := ct

		go func() {
			defer wg.Done()

			start := time.Now()

			// 为本轮刷新拷贝一个独立的RefreshProtocol
			localRefreshProto := c.RefreshProto.ShallowCopy()

			// 收集份额
			shares := make([]multiparty.RefreshShare, 0, t)
			for i := 0; i < t; i++ {
				share := <-c.RefShareChs[keyCopy]
				shares = append(shares, share)
			}
			if len(shares) != t {
				panic(fmt.Sprintf("份额数量不对,key=%d:期望 %d,收到 %d", keyCopy, t, len(shares)))
			}

			// 聚合份额
			maxLevel := params.MaxLevel()
			agg := localRefreshProto.AllocateShare(ctCopy.Level(), maxLevel)
			agg.MetaData = *ctCopy.MetaData
			for _, share := range shares {
				if err := localRefreshProto.AggregateShares(&share, &agg, &agg); err != nil {
					panic(fmt.Sprintf("AggregateShares failed: %v", err))
				}
			}

			// 最终化
			refreshed := ckks.NewCiphertext(params, 1, maxLevel)
			refreshed.Scale = params.DefaultScale()
			if err := localRefreshProto.Finalize(ctCopy, c.RefreshCRPs[keyCopy], agg, refreshed); err != nil {
				panic(fmt.Sprintf("Finalize failed at key %d: %v", keyCopy, err))
			}

			// 返回结果
			c.RefreshDone <- participant.RefreshDone{Key: keyCopy, Ciphertext: refreshed}
			fmt.Printf("[Cloud] 完成密文 %d 的自举刷新，用时 %s\n", keyCopy, time.Since(start))
		}()
	}

	wg.Wait()
	close(c.RefreshDone)
}

func runPartyBootstrap(params ckks.Parameters, N int, P []*participant.Party, C *participant.Cloud, p *participant.Party) {
	for task := range p.GenTaskQueue {
		switch t := task.(type) {
		case participant.RefreshTask:
			for key, ct := range t.Ciphertexts {
				level := ct.Level()
				maxLevel := params.MaxLevel()
				refreshShare := p.RefreshProtocol.AllocateShare(level, maxLevel)

				err := p.RefreshProtocol.GenShare(p.Sk, 128, ct, C.RefreshCRPs[key], &refreshShare)
				if err != nil {
					panic(fmt.Sprintf("Party %d GenShare failed: %v", p.ID, err))
				}

				C.RefShareChs[key] <- refreshShare
			}
			fmt.Printf("[Party %d] 完成自举份额生成\n", p.ID)
			t.Wg.Done()
		default:
			panic(fmt.Sprintf("未知任务类型: %T", t))
		}
	}
}
