package protocols

import (
	"MPHEDev/pkg/participant"
	"fmt"
	"sync"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// GeneratePublicKey 执行分布式公钥生成协议
func GeneratePublicKey(params ckks.Parameters, N int, parties []*participant.Party, cloud *participant.Cloud) {
	fmt.Println("开始生成公钥...")

	wg := new(sync.WaitGroup)

	// 启动云端聚合协程
	go runCloudPubKeyGeneration(params, N, parties, cloud)

	// 为每个参与方启动协程并分发公钥生成任务
	pubKeyTask := participant.PubKeyGenTask{
		Group: parties,
		Wg:    wg,
	}

	for _, p := range parties {
		wg.Add(1)
		p.GenTaskQueue = make(chan interface{}, 1)
		go runPartyPubKeyGeneration(params, cloud, p)
		p.GenTaskQueue <- pubKeyTask
	}

	// 等待所有协程完成任务
	wg.Wait()

	// 清理通道
	for _, p := range parties {
		close(p.GenTaskQueue)
	}
	close(cloud.PkgShareCh)

	fmt.Println("公钥生成协议完成")
}

func runCloudPubKeyGeneration(params ckks.Parameters, t int, P []*participant.Party, c *participant.Cloud) {
	pkgShare := c.PubKeyProto.AllocateShare()
	needed := t

	for PartyPkgShare := range c.PkgShareCh {
		c.PubKeyProto.AggregateShares(pkgShare, PartyPkgShare, &pkgShare)
		needed--

		if needed == 0 {
			fmt.Println("生成最终公钥中...")
			pk := rlwe.NewPublicKey(params)
			c.PubKeyProto.GenPublicKey(pkgShare, c.PkgCRP, pk)
			fmt.Println("公钥已生成，正在发送...")
			c.PkgDone <- pk
			fmt.Println("公钥发送完毕")
			break
		}
	}
	close(c.PkgDone)
	fmt.Printf("云端已聚合%d个公钥份额\n", t)
}

func runPartyPubKeyGeneration(params ckks.Parameters, C *participant.Cloud, p *participant.Party) {
	for task := range p.GenTaskQueue {
		switch t := task.(type) {
		case participant.PubKeyGenTask:
			PkgShare := p.PublicKeyGenProtocol.AllocateShare()
			p.PublicKeyGenProtocol.GenShare(p.Sk, C.PkgCRP, &PkgShare)
			C.PkgShareCh <- PkgShare
			t.Wg.Done()
		default:
			panic(fmt.Sprintf("未知任务类型: %T", t))
		}
	}
}
