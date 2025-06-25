package coordinator

import (
	"MPHEDev/cmd/Participant/types"
	"MPHEDev/cmd/Participant/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/sampling"
)

// CoordinatorClient 协调器客户端
type CoordinatorClient struct {
	baseURL       string
	client        *types.HTTPClient
	participantID int // 添加参与方ID字段
}

// NewCoordinatorClient 创建新的协调器客户端
func NewCoordinatorClient(baseURL string, client *types.HTTPClient) *CoordinatorClient {
	return &CoordinatorClient{
		baseURL:       baseURL,
		client:        client,
		participantID: 0, // 初始化为0，注册后设置
	}
}

// SetParticipantID 设置参与方ID
func (cc *CoordinatorClient) SetParticipantID(id int) {
	cc.participantID = id
}

// Register 注册到协调器
func (cc *CoordinatorClient) Register() (*types.RegisterResponse, error) {
	resp, err := cc.client.Client.Post(cc.baseURL+"/register", "application/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var regResp types.RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return nil, err
	}

	return &regResp, nil
}

// GetParams 获取CKKS参数
func (cc *CoordinatorClient) GetParams() (*ckks.Parameters, multiparty.PublicKeyGenCRP, []uint64, map[uint64]multiparty.GaloisKeyGenCRP, multiparty.RelinearizationKeyGenCRP, *sampling.KeyedPRNG, error) {
	resp, err := cc.client.Client.Get(cc.baseURL + "/params/ckks")
	if err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, nil, err
	}
	defer resp.Body.Close()

	var paramsResp types.ParamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&paramsResp); err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, nil, err
	}

	// 解析参数
	params, err := ckks.NewParametersFromLiteral(paramsResp.Params)
	if err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, nil, err
	}

	// 解析CRP
	crpBytes, err := utils.DecodeFromBase64(paramsResp.Crp)
	if err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, nil, err
	}
	var crp multiparty.PublicKeyGenCRP
	if err := utils.DecodeShare(crpBytes, &crp); err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, nil, err
	}

	// 解析伽罗瓦元素
	galEls := paramsResp.GalEls

	// 解析伽罗瓦CRPs
	galoisCRPs := make(map[uint64]multiparty.GaloisKeyGenCRP)
	for galEl, crpStr := range paramsResp.GaloisCRPs {
		crpBytes, err := utils.DecodeFromBase64(crpStr)
		if err != nil {
			return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, nil, err
		}
		var galoisCRP multiparty.GaloisKeyGenCRP
		if err := utils.DecodeShare(crpBytes, &galoisCRP); err != nil {
			return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, nil, err
		}
		galoisCRPs[galEl] = galoisCRP
	}

	// 解析重线性化CRP
	rlkCRPBytes, err := utils.DecodeFromBase64(paramsResp.RlkCRP)
	if err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, nil, err
	}
	var rlkCRP multiparty.RelinearizationKeyGenCRP
	if err := utils.DecodeShare(rlkCRPBytes, &rlkCRP); err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, nil, err
	}

	// 解析刷新CRS
	refreshCRSBytes, err := utils.DecodeFromBase64(paramsResp.RefreshCRS)
	if err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, nil, err
	}
	// 从种子重新生成KeyedPRNG
	refreshCRS, err := sampling.NewKeyedPRNG(refreshCRSBytes)
	if err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, nil, err
	}

	return &params, crp, galEls, galoisCRPs, rlkCRP, refreshCRS, nil
}

// UploadPublicKeyShare 上传公钥份额
func (cc *CoordinatorClient) UploadPublicKeyShare(shareData string) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"participant_id": cc.participantID,
		"share_data":     shareData,
	})

	resp, err := cc.client.Client.Post(cc.baseURL+"/keys/public", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上传公钥份额失败: %d", resp.StatusCode)
	}

	return nil
}

// UploadSecretKey 上传私钥
func (cc *CoordinatorClient) UploadSecretKey(secretData string) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"participant_id": cc.participantID,
		"share_data":     secretData,
	})

	resp, err := cc.client.Client.Post(cc.baseURL+"/keys/secret", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上传私钥失败: %d", resp.StatusCode)
	}

	return nil
}

// UploadGaloisKeyShare 上传伽罗瓦密钥份额
func (cc *CoordinatorClient) UploadGaloisKeyShare(galEl uint64, shareData string) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"participant_id": cc.participantID,
		"gal_el":         galEl,
		"share_data":     shareData,
	})

	resp, err := cc.client.Client.Post(cc.baseURL+"/keys/galois", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上传伽罗瓦密钥份额失败: %d", resp.StatusCode)
	}

	return nil
}

// UploadRelinearizationKeyShare 上传重线性化密钥份额
func (cc *CoordinatorClient) UploadRelinearizationKeyShare(round int, shareData string) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"participant_id": cc.participantID,
		"round":          round,
		"share_data":     shareData,
	})

	resp, err := cc.client.Client.Post(cc.baseURL+"/keys/relin", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上传重线性化密钥份额失败: %d", resp.StatusCode)
	}

	return nil
}

// GetRelinearizationKeyRound1Aggregated 获取第一轮聚合结果
func (cc *CoordinatorClient) GetRelinearizationKeyRound1Aggregated() (multiparty.RelinearizationKeyGenShare, error) {
	resp, err := cc.client.Client.Get(cc.baseURL + "/keys/relin/round1")
	if err != nil {
		return multiparty.RelinearizationKeyGenShare{}, err
	}
	defer resp.Body.Close()

	var response struct {
		Share string `json:"share"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return multiparty.RelinearizationKeyGenShare{}, err
	}

	shareBytes, err := utils.DecodeFromBase64(response.Share)
	if err != nil {
		return multiparty.RelinearizationKeyGenShare{}, err
	}

	var share multiparty.RelinearizationKeyGenShare
	if err := utils.DecodeShare(shareBytes, &share); err != nil {
		return multiparty.RelinearizationKeyGenShare{}, err
	}

	return share, nil
}

// PollStatus 轮询状态
func (cc *CoordinatorClient) PollStatus() (*types.StatusResponse, error) {
	resp, err := cc.client.Client.Get(cc.baseURL + "/setup/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var status types.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

// WaitForCompletion 等待完成
func (cc *CoordinatorClient) WaitForCompletion() error {
	for {
		status, err := cc.PollStatus()
		if err != nil {
			fmt.Println("状态查询失败:", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if status.RlkReady {
			fmt.Println("所有密钥生成完成！")
			return nil
		}

		fmt.Printf("等待密钥生成完成... (重线性化密钥: %v)\n", status.RlkReady)
		time.Sleep(2 * time.Second)
	}
}

// GetAggregatedKeys 获取聚合后的密钥
func (cc *CoordinatorClient) GetAggregatedKeys() (*types.KeysResponse, error) {
	resp, err := cc.client.Client.Get(cc.baseURL + "/keys/aggregated")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var keys types.KeysResponse
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return nil, err
	}

	return &keys, nil
}
