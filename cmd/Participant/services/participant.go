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
	Params ckks.ParametersLiteral `json:"params"`
	Crp    string                 `json:"crp"`
}

type StatusResponse struct {
	ReceivedShares  int  `json:"received_shares"`
	ReceivedSecrets int  `json:"received_secrets"`
	Total           int  `json:"total"`
	GlobalPKReady   bool `json:"global_pk_ready"`
	SkAggReady      bool `json:"sk_agg_ready"`
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

func (p *Participant) GetParams(coordinatorURL string) (*ckks.Parameters, multiparty.PublicKeyGenCRP, error) {
	resp, err := p.client.Get(coordinatorURL + "/params/ckks")
	if err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, err
	}
	defer resp.Body.Close()

	var paramsResp ParamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&paramsResp); err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, err
	}

	params, err := ckks.NewParametersFromLiteral(paramsResp.Params)
	if err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, err
	}

	crpRaw, err := utils.DecodeFromBase64(paramsResp.Crp)
	if err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, err
	}

	var crp multiparty.PublicKeyGenCRP
	if err := utils.DecodeShare(crpRaw, &crp); err != nil {
		return nil, multiparty.PublicKeyGenCRP{}, err
	}

	return &params, crp, nil
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

		fmt.Printf("当前已上传份额: %d/%d, 已上传私钥: %d, 全局公钥就绪: %v, 聚合私钥就绪: %v\n",
			status.ReceivedShares, status.Total, status.ReceivedSecrets, status.GlobalPKReady, status.SkAggReady)

		if status.GlobalPKReady && status.SkAggReady {
			fmt.Println("全局公钥和聚合私钥已聚合完成，参与方可进入下一阶段！")
			break
		}
		time.Sleep(2 * time.Second)
	}
	return nil
}
