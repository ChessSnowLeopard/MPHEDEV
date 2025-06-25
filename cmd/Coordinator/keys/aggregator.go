package keys

import (
	"MPHEDev/cmd/Coordinator/utils"
	"fmt"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// Aggregator 密钥聚合器
type Aggregator struct {
	keyManager *Manager
}

// NewAggregator 创建新的密钥聚合器
func NewAggregator(keyManager *Manager) *Aggregator {
	return &Aggregator{
		keyManager: keyManager,
	}
}

// AggregatePublicKey 聚合公钥
func (a *Aggregator) AggregatePublicKey(globalCRP multiparty.PublicKeyGenCRP) error {
	var aggShare multiparty.PublicKeyGenShare
	first := true
	proto := multiparty.NewPublicKeyGenProtocol(a.keyManager.GetParams())

	publicKeyShares := a.keyManager.GetPublicKeyShares()
	for _, data := range publicKeyShares {
		var share multiparty.PublicKeyGenShare
		if err := utils.DecodeShare(data, &share); err != nil {
			return err
		}
		if first {
			aggShare = share
			first = false
		} else {
			proto.AggregateShares(aggShare, share, &aggShare)
		}
	}

	pk := rlwe.NewPublicKey(a.keyManager.GetParams())
	proto.GenPublicKey(aggShare, globalCRP, pk)
	a.keyManager.SetGlobalPK(pk)

	fmt.Println("✓ 公钥聚合完成")
	return nil
}

// AggregateSecretKey 聚合私钥
func (a *Aggregator) AggregateSecretKey() error {
	var sks []*rlwe.SecretKey
	secretKeyShares := a.keyManager.GetSecretKeyShares()

	for _, data := range secretKeyShares {
		var sk rlwe.SecretKey
		if err := utils.DecodeShare(data, &sk); err != nil {
			return err
		}
		sks = append(sks, &sk)
	}

	skAgg := a.generateAggregatedSecretKey(a.keyManager.GetParams(), sks)
	a.keyManager.SetAggregatedSecretKey(skAgg)

	fmt.Println("✓ 私钥聚合完成")
	return nil
}

// AggregateGaloisKey 聚合伽罗瓦密钥
func (a *Aggregator) AggregateGaloisKey(galEl uint64, galoisCRP multiparty.GaloisKeyGenCRP) error {
	var aggShare multiparty.GaloisKeyGenShare
	first := true
	proto := a.keyManager.GetGaloisProto()

	// 获取该galEl的所有份额
	galoisKeyShares := a.keyManager.GetGaloisKeyShares()
	shares := galoisKeyShares[galEl]
	if len(shares) != a.keyManager.GetExpectedN() {
		return fmt.Errorf("galEl %d 的份额数量不足: %d/%d", galEl, len(shares), a.keyManager.GetExpectedN())
	}

	for _, data := range shares {
		var share multiparty.GaloisKeyGenShare
		if err := utils.DecodeShare(data, &share); err != nil {
			return err
		}
		if first {
			aggShare = share
			first = false
		} else {
			if err := proto.AggregateShares(aggShare, share, &aggShare); err != nil {
				return err
			}
		}
	}

	gk := rlwe.NewGaloisKey(a.keyManager.GetParams())
	if err := proto.GenGaloisKey(aggShare, galoisCRP, gk); err != nil {
		return err
	}

	a.keyManager.AddGaloisKey(gk)
	fmt.Printf("✓ 伽罗瓦密钥聚合完成 (galEl: %d)\n", galEl)
	return nil
}

// AggregateRelinearizationKeyRound1 聚合重线性化密钥第一轮
func (a *Aggregator) AggregateRelinearizationKeyRound1() error {
	var aggShare multiparty.RelinearizationKeyGenShare
	first := true
	proto := a.keyManager.GetRelinearizationProto()

	// 获取所有第一轮份额
	rlkShare1Map := a.keyManager.GetRelinearizationShare1Map()
	for _, data := range rlkShare1Map {
		var share multiparty.RelinearizationKeyGenShare
		if err := utils.DecodeShare(data, &share); err != nil {
			return err
		}
		if first {
			aggShare = share
			first = false
		} else {
			proto.AggregateShares(aggShare, share, &aggShare)
		}
	}

	// 存储聚合后的第一轮份额，供第二轮使用
	a.keyManager.SetRelinearizationShare1Aggregated(&aggShare)
	fmt.Println("✓ 重线性化密钥第一轮聚合完成")
	return nil
}

// AggregateRelinearizationKeyRound2 聚合重线性化密钥第二轮
func (a *Aggregator) AggregateRelinearizationKeyRound2() error {
	var aggShare multiparty.RelinearizationKeyGenShare
	first := true
	proto := a.keyManager.GetRelinearizationProto()

	// 获取所有第二轮份额
	rlkShare2Map := a.keyManager.GetRelinearizationShare2Map()
	for _, data := range rlkShare2Map {
		var share multiparty.RelinearizationKeyGenShare
		if err := utils.DecodeShare(data, &share); err != nil {
			return err
		}
		if first {
			aggShare = share
			first = false
		} else {
			proto.AggregateShares(aggShare, share, &aggShare)
		}
	}

	// 生成最终的重线性化密钥
	rlk := rlwe.NewRelinearizationKey(a.keyManager.GetParams())
	proto.GenRelinearizationKey(*a.keyManager.GetRelinearizationShare1Aggregated(), aggShare, rlk)
	a.keyManager.SetRelinearizationKey(rlk)

	fmt.Println("✓ 重线性化密钥第二轮聚合完成")
	return nil
}

// generateAggregatedSecretKey 生成聚合私钥
func (a *Aggregator) generateAggregatedSecretKey(params ckks.Parameters, sks []*rlwe.SecretKey) *rlwe.SecretKey {
	skAgg := rlwe.NewSecretKey(params)
	for _, sk := range sks {
		params.RingQP().Add(skAgg.Value, sk.Value, skAgg.Value)
	}
	return skAgg
}
