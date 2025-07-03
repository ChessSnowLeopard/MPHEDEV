package protocols

import (
	"MPHEDev/pkg/participant"
	"fmt"
	"sync"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// GenerateGaloisKeys 执行分布式伽罗瓦密钥生成协议
func GenerateGaloisKeys(params ckks.Parameters, N int, parties []*participant.Party,
	cloud *participant.Cloud, galEls []uint64) {
	fmt.Printf("开始生成%d个伽罗瓦密钥...\n", len(galEls))

	wg := new(sync.WaitGroup)

	// 启动云端聚合协程
	go runCloudGaloisKeyGeneration(params, N, galEls, cloud)

	// 启动参与方worker
	for _, p := range parties {
		p.GenTaskQueue = make(chan interface{}, 1)
		go runPartyGaloisKeyGeneration(params, cloud, p)
	}

	// 分发任务
	galoisTask := participant.GaloisKeyGenTask{
		Group:     parties,
		GaloisEls: galEls,
		Wg:        wg,
	}

	for _, p := range parties {
		wg.Add(1)
		p.GenTaskQueue <- galoisTask
	}

	// 等待所有参与方完成
	wg.Wait()

	// 关闭通道
	close(cloud.RtgShareCh)
	for _, p := range parties {
		close(p.GenTaskQueue)
	}

	fmt.Println("伽罗瓦密钥生成协议完成")
}

func runCloudGaloisKeyGeneration(params ckks.Parameters, t int, galEls []uint64, c *participant.Cloud) {
	rtgShares := make(map[uint64]*struct {
		share  multiparty.GaloisKeyGenShare
		needed int
	}, len(galEls))

	for _, galEl := range galEls {
		rtgShares[galEl] = &struct {
			share  multiparty.GaloisKeyGenShare
			needed int
		}{c.GaloisProto.AllocateShare(), t}
		rtgShares[galEl].share.GaloisElement = galEl
	}

	var j int
	for task := range c.RtgShareCh {
		acc := rtgShares[task.GalEl]
		if err := c.GaloisProto.AggregateShares(acc.share, task.Share, &acc.share); err != nil {
			panic(err)
		}
		acc.needed--
		if acc.needed == 0 {
			gk := rlwe.NewGaloisKey(params)
			if err := c.GaloisProto.GenGaloisKey(acc.share, c.GaloisCRP[task.GalEl], gk); err != nil {
				panic(err)
			}
			c.GalKeyDone <- gk
		}
		j++
	}
	close(c.GalKeyDone)
	fmt.Printf("云端为%d个伽罗瓦密钥聚合了%d个份额\n", len(galEls), j)
}

func runPartyGaloisKeyGeneration(params ckks.Parameters, C *participant.Cloud, p *participant.Party) {
	for task := range p.GenTaskQueue {
		switch t := task.(type) {
		case participant.GaloisKeyGenTask:
			for _, galEl := range t.GaloisEls {
				rtgShare := p.GaloisKeyGenProtocol.AllocateShare()
				if err := p.GaloisKeyGenProtocol.GenShare(p.Sk, galEl, C.GaloisCRP[galEl], &rtgShare); err != nil {
					panic(err)
				}
				C.RtgShareCh <- participant.RtgShareMsg{GalEl: galEl, Share: rtgShare}
			}
			t.Wg.Done()
		default:
			panic(fmt.Sprintf("未知任务类型: %T", t))
		}
	}
}
