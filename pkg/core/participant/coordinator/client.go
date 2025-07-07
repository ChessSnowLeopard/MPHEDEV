package coordinator

import (
	"MPHEDev/pkg/core/participant/types"
	"MPHEDev/pkg/core/participant/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// CoordinatorClient 协调器客户端
type CoordinatorClient struct {
	baseURL       string
	client        *types.HTTPClient
	participantID int // 添加参与方ID字段
}

// 协调器客户端"其实是指参与方（Participant）
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
func (cc *CoordinatorClient) Register(shardID string) (*types.RegisterResponse, error) {
	// 构造注册请求，包含shard_id
	reqBody := map[string]interface{}{
		"shard_id": shardID,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	resp, err := cc.client.Client.Post(cc.baseURL+"/register", "application/json", bytes.NewReader(jsonData))
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

// Unregister 注销参与方
func (cc *CoordinatorClient) Unregister(shardID string) error {
	reqBody := map[string]interface{}{
		"shard_id": shardID,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	resp, err := cc.client.Client.Post(cc.baseURL+"/unregister", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GetParams 获取CKKS参数
func (cc *CoordinatorClient) GetParams() (*types.ParamsResponse, error) {
	maxRetries := 3
	url := cc.baseURL + "/params/ckks"
	for attempt := 1; attempt <= maxRetries; attempt++ {
		fmt.Printf("尝试获取参数 (第%d次): %s\n", attempt, url)
		resp, err := cc.client.Client.Get(url)
		if err != nil {
			if attempt == maxRetries {
				return nil, fmt.Errorf("获取参数失败，已重试%d次: %v", maxRetries, err)
			}
			fmt.Printf("获取参数失败，第%d次尝试: %v，正在重试...\n", attempt, err)
			time.Sleep(2 * time.Second)
			continue
		}
		defer resp.Body.Close()

		fmt.Printf("参数请求成功，状态码: %d\n", resp.StatusCode)
		// 解析所有字段，params_literal现在是base64编码的字符串
		var raw struct {
			ParamsLiteral string   `json:"params_literal"` // 现在是base64编码的字符串
			GalEls        []uint64 `json:"gal_els"`
			CommonCRSSeed string   `json:"common_crs_seed"` // 统一的CRS种子
			DataSplitType string   `json:"data_split_type"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			if attempt == maxRetries {
				return nil, fmt.Errorf("解析参数失败，已重试%d次: %v", maxRetries, err)
			}
			fmt.Printf("解析参数失败，第%d次尝试: %v，正在重试...\n", attempt, err)
			time.Sleep(2 * time.Second)
			continue
		}

		// 解析paramsLiteral（从base64解码并JSON反序列化）
		var paramsLiteral ckks.ParametersLiteral
		if raw.ParamsLiteral != "" {
			// Base64解码
			paramsBytes, err := utils.DecodeFromBase64(raw.ParamsLiteral)
			if err != nil {
				fmt.Printf("参数base64解码失败: %v\n", err)
				if attempt == maxRetries {
					return nil, fmt.Errorf("参数base64解码失败，已重试%d次: %v", maxRetries, err)
				}
				fmt.Printf("参数base64解码失败，第%d次尝试: %v，正在重试...\n", attempt, err)
				time.Sleep(2 * time.Second)
				continue
			}

			// JSON反序列化
			if err := json.Unmarshal(paramsBytes, &paramsLiteral); err != nil {
				fmt.Printf("参数JSON反序列化失败: %v\n", err)
				if attempt == maxRetries {
					return nil, fmt.Errorf("参数JSON反序列化失败，已重试%d次: %v", maxRetries, err)
				}
				fmt.Printf("参数JSON反序列化失败，第%d次尝试: %v，正在重试...\n", attempt, err)
				time.Sleep(2 * time.Second)
				continue
			}

			fmt.Printf("成功解析paramsLiteral，LogN: %d, LogQ长度: %d\n", paramsLiteral.LogN, len(paramsLiteral.LogQ))
		}

		fmt.Printf("收到数据集划分方式: %s\n", raw.DataSplitType)

		params := &types.ParamsResponse{
			Params:        paramsLiteral,
			ParamsB64:     raw.ParamsLiteral,
			GalEls:        raw.GalEls,
			CommonCRSSeed: raw.CommonCRSSeed, // 统一的CRS种子
			DataSplitType: raw.DataSplitType,
		}

		return params, nil
	}
	return nil, fmt.Errorf("获取参数失败，已达到最大重试次数")
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
	fmt.Printf("开始请求聚合密钥...\n")

	resp, err := cc.client.Client.Get(cc.baseURL + "/keys/aggregated")
	if err != nil {
		fmt.Printf("请求聚合密钥失败: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	var keys types.KeysResponse
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		fmt.Printf("解析聚合密钥响应失败: %v\n", err)
		return nil, err
	}

	fmt.Printf("成功获取聚合密钥，包含 %d 个伽罗瓦密钥\n", len(keys.GaloisKeys))
	return &keys, nil
}
