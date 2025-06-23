package main

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/multiparty"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

type PublicKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	ShareData     string `json:"share_data"`
}
type SecretKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	ShareData     string `json:"share_data"`
}

func main() {
	// 1. 注册
	resp, err := http.Post("http://localhost:8080/register", "application/json", nil)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var regResp struct {
		ParticipantID int `json:"participant_id"`
	}
	json.NewDecoder(resp.Body).Decode(&regResp)
	myID := regResp.ParticipantID
	fmt.Println("注册成功，ID:", myID)

	// 2. 获取CKKS参数
	resp, err = http.Get("http://localhost:8080/params/ckks")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var paramsResp struct {
		Params ckks.ParametersLiteral `json:"params"`
		Crp    string                 `json:"crp"`
	}

	json.NewDecoder(resp.Body).Decode(&paramsResp)
	params, err := ckks.NewParametersFromLiteral(paramsResp.Params)
	if err != nil {
		panic(err)
	}
	crpRaw, err := base64.StdEncoding.DecodeString(paramsResp.Crp)
	if err != nil {
		panic(err)
	}
	var crp multiparty.PublicKeyGenCRP
	if err := decodeShare(crpRaw, &crp); err != nil {
		panic(err)
	}
	fmt.Println("CKKS参数:", params)

	// 3. 生成本地私钥和公钥份额
	kg := rlwe.NewKeyGenerator(params)
	sk := kg.GenSecretKeyNew()
	// 上传私钥
	skBytes, err := encodeShare(sk)
	if err != nil {
		panic(err)
	}
	skB64 := base64.StdEncoding.EncodeToString(skBytes)
	skReqBody, _ := json.Marshal(SecretKeyShare{
		ParticipantID: myID,
		ShareData:     skB64,
	})
	resp, err = http.Post("http://localhost:8080/keys/secret", "application/json", bytes.NewReader(skReqBody))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("上传私钥响应:", string(body))

	// 生成公钥份额
	proto := multiparty.NewPublicKeyGenProtocol(params)
	share := proto.AllocateShare()
	proto.GenShare(sk, crp, &share)

	// 4. gob+base64编码份额
	shareBytes, err := encodeShare(share)
	if err != nil {
		panic(err)
	}
	shareB64 := base64.StdEncoding.EncodeToString(shareBytes)

	// 5. 上传公钥份额
	reqBody, _ := json.Marshal(PublicKeyShare{
		ParticipantID: myID,
		ShareData:     shareB64,
	})
	resp, err = http.Post("http://localhost:8080/keys/public", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ = ioutil.ReadAll(resp.Body)
	fmt.Println("上传公钥份额响应:", string(body))

	// 6. 常驻在线，轮询全局公钥状态
	for {
		resp, err := http.Get("http://localhost:8080/setup/status")
		if err != nil {
			fmt.Println("状态查询失败:", err)
			time.Sleep(2 * time.Second)
			continue
		}
		var status struct {
			ReceivedShares  int  `json:"received_shares"`
			ReceivedSecrets int  `json:"received_secrets"`
			Total           int  `json:"total"`
			GlobalPKReady   bool `json:"global_pk_ready"`
			SkAggReady      bool `json:"sk_agg_ready"`
		}
		json.NewDecoder(resp.Body).Decode(&status)
		resp.Body.Close()
		fmt.Printf("当前已上传份额: %d/%d, 已上传私钥: %d, 全局公钥就绪: %v, 聚合私钥就绪: %v\n", status.ReceivedShares, status.Total, status.ReceivedSecrets, status.GlobalPKReady, status.SkAggReady)
		if status.GlobalPKReady && status.SkAggReady {
			fmt.Println("全局公钥和聚合私钥已聚合完成，参与方可进入下一阶段！")
			break
		}
		time.Sleep(2 * time.Second)
	}
}

func encodeShare(share interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(share); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
func decodeShare(data []byte, share interface{}) error {
	var buf = bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(share)
}
