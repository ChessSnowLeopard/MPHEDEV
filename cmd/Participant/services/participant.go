package services

import (
	"MPHEDev/cmd/Participant/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

type Participant struct {
	ID     int
	client *http.Client
}

type ParamsResponse struct {
	Params     ckks.ParametersLiteral `json:"params"`
	Crp        string                 `json:"crp"`
	GalEls     []uint64               `json:"gal_els"`
	GaloisCRPs map[uint64]string      `json:"galois_crps"`
	RlkCRP     string                 `json:"rlk_crp"`
}

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

func NewParticipant() *Participant {
	return &Participant{
		client: &http.Client{},
	}
}

func (p *Participant) Register(coordinatorURL string) error {
	resp, err := p.client.Post(coordinatorURL+"/register", "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var regResp struct {
		ParticipantID int `json:"participant_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return err
	}
	p.ID = regResp.ParticipantID
	return nil
}

func (p *Participant) GetParams(coordinatorURL string) (*ckks.Parameters, multiparty.PublicKeyGenCRP, []uint64, map[uint64]multiparty.GaloisKeyGenCRP, multiparty.RelinearizationKeyGenCRP, error) {
	resp, err := p.client.Get(coordinatorURL + "/params/ckks")
	if err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, err
	}
	defer resp.Body.Close()

	var paramsResp ParamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&paramsResp); err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, err
	}

	params, err := ckks.NewParametersFromLiteral(paramsResp.Params)
	if err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, err
	}

	crpRaw, err := utils.DecodeFromBase64(paramsResp.Crp)
	if err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, err
	}

	var crp multiparty.PublicKeyGenCRP
	if err := utils.DecodeShare(crpRaw, &crp); err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, err
	}

	// 解码伽罗瓦密钥CRPs
	galoisCRPs := make(map[uint64]multiparty.GaloisKeyGenCRP)
	for galEl, crpB64 := range paramsResp.GaloisCRPs {
		crpRaw, err := utils.DecodeFromBase64(crpB64)
		if err != nil {
			return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, err
		}
		var galoisCRP multiparty.GaloisKeyGenCRP
		if err := utils.DecodeShare(crpRaw, &galoisCRP); err != nil {
			return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, err
		}
		galoisCRPs[galEl] = galoisCRP
	}

	// 解码重线性化密钥CRP
	rlkCRPRaw, err := utils.DecodeFromBase64(paramsResp.RlkCRP)
	if err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, err
	}

	var rlkCRP multiparty.RelinearizationKeyGenCRP
	if err := utils.DecodeShare(rlkCRPRaw, &rlkCRP); err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, nil, nil, multiparty.RelinearizationKeyGenCRP{}, err
	}

	return &params, crp, paramsResp.GalEls, galoisCRPs, rlkCRP, nil
}

func (p *Participant) UploadPublicKeyShare(coordinatorURL string, shareData string) error {
	reqBody, _ := json.Marshal(utils.PublicKeyShare{
		ParticipantID: p.ID,
		ShareData:     shareData,
	})

	resp, err := p.client.Post(coordinatorURL+"/keys/public", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (p *Participant) UploadSecretKey(coordinatorURL string, secretData string) error {
	reqBody, _ := json.Marshal(utils.SecretKeyShare{
		ParticipantID: p.ID,
		ShareData:     secretData,
	})

	resp, err := p.client.Post(coordinatorURL+"/keys/secret", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (p *Participant) UploadGaloisKeyShare(coordinatorURL string, galEl uint64, shareData string) error {
	reqBody, _ := json.Marshal(utils.GaloisKeyShare{
		ParticipantID: p.ID,
		GalEl:         galEl,
		ShareData:     shareData,
	})

	resp, err := p.client.Post(coordinatorURL+"/keys/galois", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (p *Participant) UploadRelinearizationKeyShare(coordinatorURL string, round int, shareData string) error {
	reqBody, _ := json.Marshal(utils.RelinearizationKeyShare{
		ParticipantID: p.ID,
		Round:         round,
		ShareData:     shareData,
	})

	resp, err := p.client.Post(coordinatorURL+"/keys/relin", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (p *Participant) GetRelinearizationKeyRound1Aggregated(coordinatorURL string) (multiparty.RelinearizationKeyGenShare, error) {
	resp, err := p.client.Get(coordinatorURL + "/keys/relin/round1")
	if err != nil {
		return multiparty.RelinearizationKeyGenShare{}, err
	}
	defer resp.Body.Close()

	var response struct {
		ShareData string `json:"share_data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return multiparty.RelinearizationKeyGenShare{}, err
	}

	shareRaw, err := utils.DecodeFromBase64(response.ShareData)
	if err != nil {
		return multiparty.RelinearizationKeyGenShare{}, err
	}

	var share multiparty.RelinearizationKeyGenShare
	if err := utils.DecodeShare(shareRaw, &share); err != nil {
		return multiparty.RelinearizationKeyGenShare{}, err
	}

	return share, nil
}

func (p *Participant) PollStatus(coordinatorURL string) (*StatusResponse, error) {
	resp, err := p.client.Get(coordinatorURL + "/setup/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

func (p *Participant) WaitForCompletion(coordinatorURL string) error {
	for {
		status, err := p.PollStatus(coordinatorURL)
		if err != nil {
			fmt.Println("状态查询失败:", err)
			time.Sleep(2 * time.Second)
			continue
		}

		fmt.Printf("当前已上传份额: %d/%d, 已上传私钥: %d, 全局公钥就绪: %v, 聚合私钥就绪: %v, 伽罗瓦密钥: %d/%d, 重线性化密钥: %v\n",
			status.ReceivedShares, status.Total, status.ReceivedSecrets, status.GlobalPKReady, status.SkAggReady,
			status.CompletedGaloisKeys, status.TotalGaloisKeys, status.RlkReady)

		if status.GlobalPKReady && status.SkAggReady && status.CompletedGaloisKeys == status.TotalGaloisKeys && status.RlkReady {
			fmt.Println("全局公钥、聚合私钥、所有伽罗瓦密钥和重线性化密钥已聚合完成，参与方可进入下一阶段！")
			break
		}
		time.Sleep(2 * time.Second)
	}
	return nil
}
