package server

import (
	"MPHEDev/cmd/Participant/crypto"
	"MPHEDev/cmd/Participant/utils"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// Handlers HTTP处理器集合
type Handlers struct {
	keyManager        *crypto.KeyManager
	decryptionService *crypto.DecryptionService
}

// NewHandlers 创建新的处理器集合
func NewHandlers(keyManager *crypto.KeyManager, decryptionService *crypto.DecryptionService) *Handlers {
	return &Handlers{
		keyManager:        keyManager,
		decryptionService: decryptionService,
	}
}

// GetHandlers 获取所有处理器
func (h *Handlers) GetHandlers() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/health":          h.handleHealth,
		"/bootstrap":       h.handleBootstrap,
		"/partial_decrypt": h.handlePartialDecrypt,
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

// handleReceiveKeys 接收密钥处理器
func (h *Handlers) handleReceiveKeys(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Params     interface{}   `json:"params"`
		PubKey     interface{}   `json:"pub_key"`
		RelineKey  interface{}   `json:"reline_key"`
		GaloisKeys []interface{} `json:"galois_keys"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求解析失败", http.StatusBadRequest)
		return
	}

	// 这里应该实现密钥接收和设置逻辑
	// 由于密钥结构复杂，这里只是示例框架

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "keys_received",
	})
}
