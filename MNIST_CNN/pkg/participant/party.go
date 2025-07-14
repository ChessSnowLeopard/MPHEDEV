package participant

import (
	"sync"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/multiparty/mpckks"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// Party 表示一个参与方
type Party struct {
	multiparty.PublicKeyGenProtocol
	multiparty.GaloisKeyGenProtocol
	multiparty.RelinearizationKeyGenProtocol
	mpckks.RefreshProtocol

	// 重线性化密钥过程需要的临时密钥和份额
	RlkEphSk  *rlwe.SecretKey // 重线性化临时密钥
	RlkShare1 multiparty.RelinearizationKeyGenShare
	RlkShare2 multiparty.RelinearizationKeyGenShare

	// 协同解密相关
	DecryptProtocol multiparty.KeySwitchProtocol
	DecryptShareChs map[uint64]chan multiparty.KeySwitchShare
	DecryptDone     chan DecryptDone
	ToDecryptCts    map[uint64]*rlwe.Ciphertext

	ID int
	Sk *rlwe.SecretKey // 私有密钥
	//任务完成后收获的密钥和参数
	Pk     *rlwe.PublicKey
	Rlk    *rlwe.RelinearizationKey
	Gks    []*rlwe.GaloisKey
	Params ckks.Parameters //  同态加密参数

	// 任务队列
	GenTaskQueue chan interface{}
}

// 任务类型定义
type GaloisKeyGenTask struct {
	Group     []*Party
	GaloisEls []uint64
	Wg        *sync.WaitGroup
}

type PubKeyGenTask struct {
	Group []*Party
	Wg    *sync.WaitGroup
}

type RlKeyGenTask struct {
	OneOrTwo bool
	Group    []*Party
	Wg       *sync.WaitGroup
}

type RefreshTask struct {
	Ciphertexts map[uint64]*rlwe.Ciphertext
	Wg          *sync.WaitGroup
}

type DecryptTask struct {
	Ciphertext *rlwe.Ciphertext
	Key        uint64
}

// 消息类型定义
type DecryptShareMsg struct {
	Key   uint64
	Share multiparty.KeySwitchShare
}

type DecryptDone struct {
	Key       uint64
	Plaintext *rlwe.Plaintext
}

// NewParty 创建新的参与方
func NewParty(id int, params ckks.Parameters, kg *rlwe.KeyGenerator) *Party {
	return &Party{
		PublicKeyGenProtocol:          multiparty.NewPublicKeyGenProtocol(params),
		GaloisKeyGenProtocol:          multiparty.NewGaloisKeyGenProtocol(params),
		RelinearizationKeyGenProtocol: multiparty.NewRelinearizationKeyGenProtocol(params),
		ID:                            id,
		Sk:                            kg.GenSecretKeyNew(),
		Params:                        params,
		DecryptShareChs:               make(map[uint64]chan multiparty.KeySwitchShare),
		ToDecryptCts:                  make(map[uint64]*rlwe.Ciphertext),
	}
}
