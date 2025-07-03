package server

import (
	"MPHEDev/pkg/core/participant/crypto"
	"MPHEDev/pkg/core/participant/types"
	"MPHEDev/pkg/core/participant/utils"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// Handlers HTTP处理器集合
type Handlers struct {
	keyManager        *crypto.KeyManager
	decryptionService *crypto.DecryptionService
	refreshService    *crypto.RefreshService
}

// NewHandlers 创建新的处理器集合
func NewHandlers(keyManager *crypto.KeyManager, decryptionService *crypto.DecryptionService, refreshService *crypto.RefreshService) *Handlers {
	return &Handlers{
		keyManager:        keyManager,
		decryptionService: decryptionService,
		refreshService:    refreshService,
	}
}

// GetHandlers 获取所有处理器
func (h *Handlers) GetHandlers() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/health":          h.handleHealth,
		"/bootstrap":       h.handleBootstrap,
		"/partial_decrypt": h.handlePartialDecrypt,
		"/partial_refresh": h.handlePartialRefresh,
		"/keys/receive":    h.handleReceiveKeys,
	}
}

// handleHealth 健康检查处理器
func (h *Handlers) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"ready":  h.keyManager.IsReady(),
	})
}

// handleBootstrap 启动处理器
func (h *Handlers) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "bootstrap_complete",
	})
}

// handlePartialDecrypt 部分解密处理器
func (h *Handlers) handlePartialDecrypt(w http.ResponseWriter, r *http.Request) {
	if !h.keyManager.IsReady() {
		http.Error(w, "密钥未准备就绪", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		TaskID     string `json:"task_id"`
		Ciphertext string `json:"ciphertext"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求解析失败", http.StatusBadRequest)
		return
	}

	// 解码密文
	ctBytes, err := utils.DecodeFromBase64(req.Ciphertext)
	if err != nil {
		http.Error(w, "密文解码失败", http.StatusBadRequest)
		return
	}

	var ct rlwe.Ciphertext
	if err := utils.DecodeShare(ctBytes, &ct); err != nil {
		http.Error(w, "密文反序列化失败", http.StatusBadRequest)
		return
	}

	// 生成解密份额
	share, err := h.decryptionService.GeneratePartialDecryptShare(&ct, req.TaskID)
	if err != nil {
		http.Error(w, fmt.Sprintf("生成解密份额失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"share": share,
	})
}

// handleReceiveKeys 处理接收密钥请求
func (h *Handlers) handleReceiveKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var keysData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&keysData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 解析并设置密钥
	if pubKeyStr, ok := keysData["pub_key"].(string); ok {
		pubKeyBytes, err := utils.DecodeFromBase64(pubKeyStr)
		if err == nil {
			var pubKey rlwe.PublicKey
			if err := utils.DecodeShare(pubKeyBytes, &pubKey); err == nil {
				h.keyManager.SetPublicKey(&pubKey)
			}
		}
	}

	if relineKeyStr, ok := keysData["reline_key"].(string); ok {
		relineKeyBytes, err := utils.DecodeFromBase64(relineKeyStr)
		if err == nil {
			var relineKey rlwe.RelinearizationKey
			if err := utils.DecodeShare(relineKeyBytes, &relineKey); err == nil {
				h.keyManager.SetRelinearizationKey(&relineKey)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "keys_received"})
}

// handlePartialRefresh 处理部分刷新请求
func (h *Handlers) handlePartialRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 解析密文
	ctBytes, err := utils.DecodeFromBase64(req.Ciphertext)
	if err != nil {
		http.Error(w, "Invalid ciphertext", http.StatusBadRequest)
		return
	}

	var ct rlwe.Ciphertext
	if err := utils.DecodeShare(ctBytes, &ct); err != nil {
		http.Error(w, "Failed to decode ciphertext", http.StatusBadRequest)
		return
	}

	// 生成刷新份额
	share, err := h.refreshService.GenerateRefreshShare(&ct, req.TaskID)
	if err != nil {
		http.Error(w, "Failed to generate refresh share", http.StatusInternalServerError)
		return
	}

	// 序列化份额
	shareBytes, err := utils.EncodeShare(share)
	if err != nil {
		http.Error(w, "Failed to encode share", http.StatusInternalServerError)
		return
	}

	shareB64 := utils.EncodeToBase64(shareBytes)

	// 返回份额
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(types.RefreshShareResponse{
		Share: shareB64,
	})
}
