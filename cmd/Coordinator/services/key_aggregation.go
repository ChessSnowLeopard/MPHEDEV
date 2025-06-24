package services

import (
	"MPHEDev/cmd/Coordinator/utils"

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

func generateAggregatedSecretKey(params ckks.Parameters, sks []*rlwe.SecretKey) *rlwe.SecretKey {
	skAgg := rlwe.NewSecretKey(params)
	for _, sk := range sks {
		params.RingQP().Add(skAgg.Value, sk.Value, skAgg.Value)
	}
	return skAgg
}
