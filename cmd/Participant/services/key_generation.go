package services

import (
	"MPHEDev/cmd/Participant/utils"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

type KeyGenerator struct {
	params ckks.Parameters
	crp    multiparty.PublicKeyGenCRP
}

func NewKeyGenerator(params ckks.Parameters, crp multiparty.PublicKeyGenCRP) *KeyGenerator {
	return &KeyGenerator{
		params: params,
		crp:    crp,
	}
}

func (kg *KeyGenerator) GenerateKeys() (*rlwe.SecretKey, multiparty.PublicKeyGenShare, error) {
	// 生成本地私钥
	keyGen := rlwe.NewKeyGenerator(kg.params)
	sk := keyGen.GenSecretKeyNew()

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
