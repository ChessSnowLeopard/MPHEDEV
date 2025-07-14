package utils

import (
	"github.com/tuneinsight/lattigo/v6/ring"
)

type ParticipantInfo struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

type PeerInfo struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

type PublicKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	ShareData     string `json:"share_data"`
}

type SecretKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	ShareData     string `json:"share_data"`
}

type GaloisKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	GalEl         uint64 `json:"gal_el"`
	ShareData     string `json:"share_data"`
}

type RelinearizationKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	Round         int    `json:"round"`
	ShareData     string `json:"share_data"`
}

// CustomParametersLiteral 自定义参数结构体，用于正确的JSON序列化
type CustomParametersLiteral struct {
	LogN            int                   `json:"LogN"`
	LogNthRoot      int                   `json:"LogNthRoot"`
	LogQ            []int                 `json:"LogQ"` // 正确的JSON标签
	LogP            []int                 `json:"LogP"` // 正确的JSON标签
	Xe              ring.DiscreteGaussian `json:"Xe"`
	Xs              ring.Ternary          `json:"Xs"`
	RingType        ring.Type             `json:"RingType"`
	LogDefaultScale int                   `json:"LogDefaultScale"`
}
