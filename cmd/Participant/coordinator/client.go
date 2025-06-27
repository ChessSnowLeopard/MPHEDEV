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
)

// CoordinatorClient 协调器客户端
type CoordinatorClient struct {
	baseURL       string
	client        *types.HTTPClient
	participantID int // 添加参与方ID字段
}

// 协调器客户端”其实是指参与方（Participant）
// 用来和协调器（Coordinator）通信的一个客户端工具
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
func (cc *CoordinatorClient) GetParams() (*types.ParamsResponse, error) {
	resp, err := cc.client.Client.Get(cc.baseURL + "/params/ckks")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var params types.ParamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&params); err != nil {
		return nil, err
	}

	return &params, nil
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
