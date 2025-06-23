package protocols

import (
	"MPHEDev/pkg/participant"
	"fmt"
	"sync"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// GenerateRelinearizationKey 执行分布式重线性化密钥生成协议
func GenerateRelinearizationKey(params ckks.Parameters, N int, parties []*participant.Party, cloud *participant.Cloud) {
	fmt.Println("开始生成重线性化密钥...")

	// 启动云端聚合协程
	go runCloudRelinKeyGeneration(params, N, parties, cloud)

	// 启动每个参与方的工作协程
	for _, p := range parties {
		p.GenTaskQueue = make(chan interface{}, 1)
		go runPartyRelinKeyGeneration(params, cloud, p)
	}

	// 第一轮任务
	wg1 := new(sync.WaitGroup)
	rlkTask1 := participant.RlKeyGenTask{
		OneOrTwo: false,
		Group:    parties,
		Wg:       wg1,
	}

	for _, p := range parties {
		wg1.Add(1)
		p.GenTaskQueue <- rlkTask1
	}
	wg1.Wait()

	// 第二轮任务
	wg2 := new(sync.WaitGroup)
	rlkTask2 := participant.RlKeyGenTask{
		OneOrTwo: true,
		Group:    parties,
		Wg:       wg2,
	}

	for _, p := range parties {
		wg2.Add(1)
		p.GenTaskQueue <- rlkTask2
	}
	wg2.Wait()

	// 清理
	close(cloud.RlkShareCh)
	close(cloud.RlkCombinedShareChan)
	for _, p := range parties {
		close(p.GenTaskQueue)
	}

	fmt.Println("重线性化密钥生成协议完成")
}

func runCloudRelinKeyGeneration(params ckks.Parameters, t int, P []*participant.Party, c *participant.Cloud) {
	var RkgCombined1, RkgCombined2 multiparty.RelinearizationKeyGenShare
	_, RkgCombined1, RkgCombined2 = c.RelineKeyProto.AllocateShare()

	received := 0
	round := 1

	for share := range c.RlkShareCh {
		if round == 1 {
			c.RelineKeyProto.AggregateShares(share, RkgCombined1, &RkgCombined1)
			received++
			if received == t {
				// 广播给所有参与方
				for i := 0; i < t; i++ {
					c.RlkCombinedShareChan <- &RkgCombined1
				}
				round = 2
				received = 0
			}
		} else if round == 2 {
			c.RelineKeyProto.AggregateShares(share, RkgCombined2, &RkgCombined2)
			received++
			if received == t {
				rlk := rlwe.NewRelinearizationKey(params)
				c.RelineKeyProto.GenRelinearizationKey(RkgCombined1, RkgCombined2, rlk)
				c.RlkDone <- rlk
				close(c.RlkDone)
				fmt.Printf("云端为%d个参与方生成了重线性化密钥\n", t)
				break
			}
		}
	}
}

func runPartyRelinKeyGeneration(params ckks.Parameters, C *participant.Cloud, p *participant.Party) {
	if p.RlkEphSk == nil {
		p.RlkEphSk, p.RlkShare1, p.RlkShare2 = p.RelinearizationKeyGenProtocol.AllocateShare()
	}

	for task := range p.GenTaskQueue {
		switch t := task.(type) {
		case participant.RlKeyGenTask:
			if t.OneOrTwo {
				// 第二轮：等待云端聚合的share1
				RlkCombined := <-C.RlkCombinedShareChan
				p.RelinearizationKeyGenProtocol.GenShareRoundTwo(p.RlkEphSk, p.Sk, *RlkCombined, &p.RlkShare2)
				C.RlkShareCh <- p.RlkShare2
			} else {
				// 第一轮
				p.RelinearizationKeyGenProtocol.GenShareRoundOne(p.Sk, C.RlkCRP, p.RlkEphSk, &p.RlkShare1)
				C.RlkShareCh <- p.RlkShare1
			}
			t.Wg.Done()
		default:
			panic(fmt.Sprintf("未知任务类型: %T", t))
		}
	}
}
