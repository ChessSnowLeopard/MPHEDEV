package types

import (
	"net/http"
	"sync"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// PeerInfo 参与方信息
type PeerInfo struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Port int `json:"port"`
}

// RegisterResponse 注册响应
type RegisterResponse struct {
	ParticipantID int `json:"participant_id"`
}

// ParamsResponse 参数响应
type ParamsResponse struct {
	Params        ckks.ParametersLiteral `json:"params"`
	Crp           string                 `json:"crp"`
	GalEls        []uint64               `json:"gal_els"`
	GaloisCRPs    map[string]string      `json:"galois_crps"`
	RlkCRP        string                 `json:"rlk_crp"`
	RefreshCRS    string                 `json:"refresh_crs"`
	DataSplitType string                 `json:"data_split_type"` // 数据集划分方式
}

// StatusResponse 状态响应
type StatusResponse struct {
	ReceivedShares      int  `json:"received_shares"`
	ReceivedSecrets     int  `json:"received_secrets"`
	Total               int  `json:"total"`
	GlobalPKReady       bool `json:"global_pk_ready"`
	SkAggReady          bool `json:"sk_agg_ready"`
	GaloisKeysReady     int  `json:"galois_keys_ready"`
	TotalGaloisKeys     int  `json:"total_galois_keys"`
	CompletedGaloisKeys int  `json:"completed_galois_keys"`
	RlkRound1Ready      bool `json:"rlk_round1_ready"`
	RlkRound2Ready      bool `json:"rlk_round2_ready"`
	RlkReady            bool `json:"rlk_ready"`
}

// KeysResponse 聚合密钥响应
type KeysResponse struct {
	PubKey     string            `json:"pub_key"`
	RelineKey  string            `json:"reline_key"`
	GaloisKeys map[uint64]string `json:"galois_keys"`
}

// Participant 参与方主结构体
type Participant struct {
	ID     int
	Port   int
	Client *HTTPClient

	// 网络相关
	PeerManager      *PeerManager
	HeartbeatManager *HeartbeatManager
	HTTPServer       *HTTPServer

	// 加密相关
	KeyManager        *KeyManager
	DecryptionService *DecryptionService

	// 协调器客户端
	CoordinatorClient *CoordinatorClient

	// 状态管理
	Ready   bool
	ReadyCh chan struct{}
}

// HTTPClient HTTP客户端
type HTTPClient struct {
	Client *http.Client
}

// PeerManager P2P网络管理
type PeerManager struct {
	Peers map[int]string // ID -> URL映射
	mu    sync.RWMutex
}

// HeartbeatManager 心跳和在线状态管理
type HeartbeatManager struct {
	coordinatorURL     string
	heartbeatTicker    *time.Ticker
	heartbeatStopCh    chan struct{}
	onlinePeers        map[int]string
	lastPeerUpdate     time.Time
	peerUpdateInterval time.Duration
	silentMode         bool
	client             *HTTPClient
}

// HTTPServer HTTP服务器
type HTTPServer struct {
	Server *http.Server
	Port   int
}

// KeyManager 密钥管理
type KeyManager struct {
	Params          ckks.Parameters
	TotalGaloisKeys int
	PubKey          *rlwe.PublicKey
	RelineKey       *rlwe.RelinearizationKey
	GaloisKeys      []*rlwe.GaloisKey
	Sk              *rlwe.SecretKey
}

// DecryptionService 解密服务
type DecryptionService struct {
	keyManager *KeyManager
	client     *HTTPClient
}

// CoordinatorClient 协调器客户端
type CoordinatorClient struct {
	baseURL string
	client  *HTTPClient
}

// RefreshTask 刷新任务
type RefreshTask struct {
	TaskID     string `json:"task_id"`
	Ciphertext string `json:"ciphertext"` // base64编码的密文
}

// RefreshShareResponse 刷新份额响应
type RefreshShareResponse struct {
	Share string `json:"share"` // base64编码的刷新份额
}

// RefreshRequest 刷新请求
type RefreshRequest struct {
	TaskID     string `json:"task_id"`
	Ciphertext string `json:"ciphertext"`
}
