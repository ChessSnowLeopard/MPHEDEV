package services

import (
	"MPHEDev/cmd/Coordinator/utils"

	"fmt"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

func (c *Coordinator) AggregatePublicKey() error {
	var aggShare multiparty.PublicKeyGenShare
	first := true
	proto := multiparty.NewPublicKeyGenProtocol(c.params)

	for _, data := range c.publicKeyShares {
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

	pk := rlwe.NewPublicKey(c.params)
	proto.GenPublicKey(aggShare, c.globalCRP, pk)
	c.globalPK = pk
	return nil
}

func (c *Coordinator) AggregateSecretKey() error {
	var sks []*rlwe.SecretKey
	for _, data := range c.secretKeyShares {
		var sk rlwe.SecretKey
		if err := utils.DecodeShare(data, &sk); err != nil {
			return err
		}
		sks = append(sks, &sk)
	}
	c.skAgg = generateAggregatedSecretKey(c.params, sks)
	return nil
}

func (c *Coordinator) AggregateGaloisKey(galEl uint64) error {
	var aggShare multiparty.GaloisKeyGenShare
	first := true
	proto := multiparty.NewGaloisKeyGenProtocol(c.params)

	// 获取该galEl的所有份额
	shares := c.galoisKeyShares[galEl]
	if len(shares) != c.expectedN {
		return fmt.Errorf("galEl %d 的份额数量不足: %d/%d", galEl, len(shares), c.expectedN)
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

	gk := rlwe.NewGaloisKey(c.params)
	if err := proto.GenGaloisKey(aggShare, c.galoisCRPs[galEl], gk); err != nil {
		return err
	}

	c.galoisKeys = append(c.galoisKeys, gk)
	return nil
}

func generateAggregatedSecretKey(params ckks.Parameters, sks []*rlwe.SecretKey) *rlwe.SecretKey {
	skAgg := rlwe.NewSecretKey(params)
	for _, sk := range sks {
		params.RingQP().Add(skAgg.Value, sk.Value, skAgg.Value)
	}
	return skAgg
}

func (c *Coordinator) AggregateRelinearizationKeyRound1() error {
	var aggShare multiparty.RelinearizationKeyGenShare
	first := true

	// 获取所有第一轮份额
	for _, data := range c.rlkShare1Map {
		var share multiparty.RelinearizationKeyGenShare
		if err := utils.DecodeShare(data, &share); err != nil {
			return err
		}
		if first {
			aggShare = share
			first = false
		} else {
			c.rlkProto.AggregateShares(aggShare, share, &aggShare)
		}
	}

	// 存储聚合后的第一轮份额，供第二轮使用
	c.rlkShare1Aggregated = &aggShare
	return nil
}

func (c *Coordinator) AggregateRelinearizationKeyRound2() error {
	var aggShare multiparty.RelinearizationKeyGenShare
	first := true

	// 获取所有第二轮份额
	for _, data := range c.rlkShare2Map {
		var share multiparty.RelinearizationKeyGenShare
		if err := utils.DecodeShare(data, &share); err != nil {
			return err
		}
		if first {
			aggShare = share
			first = false
		} else {
			c.rlkProto.AggregateShares(aggShare, share, &aggShare)
		}
	}

	// 生成最终的重线性化密钥
	rlk := rlwe.NewRelinearizationKey(c.params)
	c.rlkProto.GenRelinearizationKey(*c.rlkShare1Aggregated, aggShare, rlk)
	c.rlk = rlk
	return nil
}
