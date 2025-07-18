package participant

import (
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/multiparty/mpckks"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

// Cloud 表示云端协调者
type Cloud struct {
	// 公钥生成相关
	PubKeyProto multiparty.PublicKeyGenProtocol
	PkgCRP      multiparty.PublicKeyGenCRP
	PkgShareCh  chan multiparty.PublicKeyGenShare
	PkgDone     chan *rlwe.PublicKey

	// 重线性化密钥生成相关
	RelineKeyProto       multiparty.RelinearizationKeyGenProtocol
	RlkCRP               multiparty.RelinearizationKeyGenCRP
	RlkShareCh           chan multiparty.RelinearizationKeyGenShare
	RlkCombinedShareChan chan *multiparty.RelinearizationKeyGenShare
	RlkDone              chan *rlwe.RelinearizationKey

	// 伽罗瓦密钥生成相关
	GaloisProto multiparty.GaloisKeyGenProtocol
	GaloisCRP   map[uint64]multiparty.GaloisKeyGenCRP
	RtgShareCh  chan RtgShareMsg
	GalKeyDone  chan *rlwe.GaloisKey

	// 自举协议相关
	RefreshProto mpckks.RefreshProtocol
	RefreshCRPs  map[uint64]multiparty.KeySwitchCRP
	RefShareChs  map[uint64]chan multiparty.RefreshShare
	RefreshDone  chan RefreshDone
	ToRefreshCts map[uint64]*rlwe.Ciphertext
}

// 消息类型定义
type RefreshShareMsg struct {
	Key   uint64
	Share multiparty.RefreshShare
}

type RtgShareMsg struct {
	GalEl uint64
	Share multiparty.GaloisKeyGenShare
}

type RefreshDone struct {
	Key        uint64
	Ciphertext *rlwe.Ciphertext
}

// NewCloud 创建新的云端实例
func NewCloud(params ckks.Parameters, N int, galEls []uint64, crs *sampling.KeyedPRNG) *Cloud {
	cloudProtocol := multiparty.NewPublicKeyGenProtocol(params)
	galoisProto := multiparty.NewGaloisKeyGenProtocol(params)
	relinProtocol := multiparty.NewRelinearizationKeyGenProtocol(params)

	// 生成Galois CRP
	rtgCRP := make(map[uint64]multiparty.GaloisKeyGenCRP)
	for _, galEl := range galEls {
		rtgCRP[galEl] = galoisProto.SampleCRP(crs)
	}

	return &Cloud{
		// 公钥协议部分
		PubKeyProto: cloudProtocol,
		PkgCRP:      cloudProtocol.SampleCRP(crs),
		PkgShareCh:  make(chan multiparty.PublicKeyGenShare, N),
		PkgDone:     make(chan *rlwe.PublicKey, 1),

		// 重线性化密钥协议部分
		RelineKeyProto:       relinProtocol,
		RlkCRP:               relinProtocol.SampleCRP(crs),
		RlkShareCh:           make(chan multiparty.RelinearizationKeyGenShare, N),
		RlkDone:              make(chan *rlwe.RelinearizationKey, 1),
		RlkCombinedShareChan: make(chan *multiparty.RelinearizationKeyGenShare, N),

		// Galois Key协议部分
		GaloisProto: galoisProto,
		GaloisCRP:   rtgCRP,
		RtgShareCh:  make(chan RtgShareMsg, len(galEls)*N),
		GalKeyDone:  make(chan *rlwe.GaloisKey, len(galEls)),
	}
}
