package services

import (
	"MPHEDev/pkg/core/participant/utils"
	"fmt"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

type KeyGenerator struct {
	params      ckks.Parameters
	crp         multiparty.PublicKeyGenCRP
	galEls      []uint64
	galoisCRPs  map[uint64]multiparty.GaloisKeyGenCRP
	sk          *rlwe.SecretKey
	galoisProto multiparty.GaloisKeyGenProtocol

	// 重线性化密钥相关
	rlkCRP    multiparty.RelinearizationKeyGenCRP
	rlkProto  multiparty.RelinearizationKeyGenProtocol
	rlkEphSk  *rlwe.SecretKey
	rlkShare1 multiparty.RelinearizationKeyGenShare
	rlkShare2 multiparty.RelinearizationKeyGenShare
}

func NewKeyGenerator(params ckks.Parameters, crp *multiparty.PublicKeyGenCRP, galEls []uint64, galoisCRPs map[uint64]multiparty.GaloisKeyGenCRP, rlkCRP *multiparty.RelinearizationKeyGenCRP) *KeyGenerator {
	return &KeyGenerator{
		params:      params,
		crp:         *crp,
		galEls:      galEls,
		galoisCRPs:  galoisCRPs,
		galoisProto: multiparty.NewGaloisKeyGenProtocol(params),
		rlkCRP:      *rlkCRP,
		rlkProto:    multiparty.NewRelinearizationKeyGenProtocol(params),
	}
}

func (kg *KeyGenerator) GenerateKeys() (*rlwe.SecretKey, multiparty.PublicKeyGenShare, error) {
	// 生成本地私钥
	keyGen := rlwe.NewKeyGenerator(kg.params)
	sk := keyGen.GenSecretKeyNew()
	kg.sk = sk

	// 生成公钥份额
	proto := multiparty.NewPublicKeyGenProtocol(kg.params)
	share := proto.AllocateShare()
	proto.GenShare(sk, kg.crp, &share)

	return sk, share, nil
}

func (kg *KeyGenerator) EncodeSecretKey(sk *rlwe.SecretKey) (string, error) {
	skBytes, err := utils.EncodeShare(sk)
	if err != nil {
		return "", err
	}
	return utils.EncodeToBase64(skBytes), nil
}

func (kg *KeyGenerator) EncodePublicKeyShare(share multiparty.PublicKeyGenShare) (string, error) {
	shareBytes, err := utils.EncodeShare(share)
	if err != nil {
		return "", err
	}
	return utils.EncodeToBase64(shareBytes), nil
}

func (kg *KeyGenerator) GenerateGaloisKeyShares() (map[uint64]multiparty.GaloisKeyGenShare, error) {
	if kg.sk == nil {
		return nil, fmt.Errorf("私钥未生成，请先调用GenerateKeys")
	}

	shares := make(map[uint64]multiparty.GaloisKeyGenShare)
	for _, galEl := range kg.galEls {
		share := kg.galoisProto.AllocateShare()
		if err := kg.galoisProto.GenShare(kg.sk, galEl, kg.galoisCRPs[galEl], &share); err != nil {
			return nil, err
		}
		shares[galEl] = share
	}
	return shares, nil
}

func (kg *KeyGenerator) EncodeGaloisKeyShare(share multiparty.GaloisKeyGenShare) (string, error) {
	shareBytes, err := utils.EncodeShare(share)
	if err != nil {
		return "", err
	}
	return utils.EncodeToBase64(shareBytes), nil
}

func (kg *KeyGenerator) GenerateRelinearizationKeyRound1() error {
	if kg.sk == nil {
		return fmt.Errorf("私钥未生成，请先调用GenerateKeys")
	}

	// 分配重线性化密钥份额
	kg.rlkEphSk, kg.rlkShare1, kg.rlkShare2 = kg.rlkProto.AllocateShare()

	// 生成第一轮份额
	kg.rlkProto.GenShareRoundOne(kg.sk, kg.rlkCRP, kg.rlkEphSk, &kg.rlkShare1)
	return nil
}

func (kg *KeyGenerator) GenerateRelinearizationKeyRound2(aggregatedShare1 multiparty.RelinearizationKeyGenShare) error {
	if kg.rlkEphSk == nil {
		return fmt.Errorf("第一轮份额未生成，请先调用GenerateRelinearizationKeyRound1")
	}

	// 检查aggregatedShare1的关键字段
	if aggregatedShare1.Value == nil {
		return fmt.Errorf("聚合份额的Value字段为空")
	}

	// 生成第二轮份额
	kg.rlkProto.GenShareRoundTwo(kg.rlkEphSk, kg.sk, aggregatedShare1, &kg.rlkShare2)
	return nil
}

func (kg *KeyGenerator) EncodeRelinearizationKeyShare(round int) (string, error) {
	var share multiparty.RelinearizationKeyGenShare
	if round == 1 {
		share = kg.rlkShare1
	} else if round == 2 {
		share = kg.rlkShare2
	} else {
		return "", fmt.Errorf("无效的轮次: %d", round)
	}

	shareBytes, err := utils.EncodeShare(share)
	if err != nil {
		return "", err
	}
	return utils.EncodeToBase64(shareBytes), nil
}
