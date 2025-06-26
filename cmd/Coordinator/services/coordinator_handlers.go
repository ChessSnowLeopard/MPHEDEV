package services

import (
	"MPHEDev/cmd/Coordinator/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HTTP处理器方法
func (c *Coordinator) registerHandler(ctx *gin.Context) {
	id := c.RegisterParticipant()
	ctx.JSON(http.StatusOK, gin.H{"participant_id": id})
}

func (c *Coordinator) getCKKSParamsHandler(ctx *gin.Context) {
	params, crp, galEls, galoisCRPs, rlkCRP, refreshCRS := c.GetParams()
	ctx.JSON(http.StatusOK, gin.H{
		"params":      params,
		"crp":         crp,
		"gal_els":     galEls,
		"galois_crps": galoisCRPs,
		"rlk_crp":     rlkCRP,
		"refresh_crs": refreshCRS,
	})
}

func (c *Coordinator) postPublicKeyHandler(ctx *gin.Context) {
	var req utils.PublicKeyShare
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	data, err := utils.DecodeFromBase64(req.ShareData)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid share data"})
		return
	}

	if err := c.AddPublicKeyShare(req.ParticipantID, data); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "public key share added"})
}

func (c *Coordinator) postSecretKeyHandler(ctx *gin.Context) {
	var req utils.SecretKeyShare
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	data, err := utils.DecodeFromBase64(req.ShareData)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid share data"})
		return
	}

	if err := c.AddSecretKey(req.ParticipantID, data); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "secret key added"})
}

func (c *Coordinator) postGaloisKeyHandler(ctx *gin.Context) {
	var req utils.GaloisKeyShare
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	data, err := utils.DecodeFromBase64(req.ShareData)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid share data"})
		return
	}

	if err := c.AddGaloisKeyShare(req.ParticipantID, req.GalEl, data); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "galois key share added"})
}

func (c *Coordinator) postRelinearizationKeyHandler(ctx *gin.Context) {
	var req utils.RelinearizationKeyShare
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	data, err := utils.DecodeFromBase64(req.ShareData)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid share data"})
		return
	}

	if err := c.AddRelinearizationKeyShare(req.ParticipantID, req.Round, data); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "relinearization key share added"})
}

func (c *Coordinator) getRelinearizationKeyRound1AggregatedHandler(ctx *gin.Context) {
	share, err := c.GetRelinearizationKeyRound1Aggregated()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"share": share})
}

func (c *Coordinator) getParticipantsHandler(ctx *gin.Context) {
	participants := c.GetParticipants()
	ctx.JSON(http.StatusOK, participants)
}

func (c *Coordinator) getSetupStatusHandler(ctx *gin.Context) {
	status := c.GetStatus()
	ctx.JSON(http.StatusOK, status)
}

func (c *Coordinator) reportURLHandler(ctx *gin.Context) {
	var req utils.PeerInfo
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := c.AddParticipantURL(req.ID, req.URL); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "url_reported"})
}

func (c *Coordinator) getParticipantsListHandler(ctx *gin.Context) {
	participants := c.GetAllParticipantURLs()
	ctx.JSON(http.StatusOK, participants)
}

func (c *Coordinator) heartbeatHandler(ctx *gin.Context) {
	var req struct {
		ParticipantID int `json:"participant_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := c.UpdateHeartbeat(req.ParticipantID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "heartbeat_updated"})
}

func (c *Coordinator) getOnlineParticipantsHandler(ctx *gin.Context) {
	onlineParticipants := c.GetOnlineParticipants()
	ctx.JSON(http.StatusOK, onlineParticipants)
}

func (c *Coordinator) getOnlineStatusHandler(ctx *gin.Context) {
	status := c.GetOnlineStatus()
	ctx.JSON(http.StatusOK, status)
}

func (c *Coordinator) testAllKeysHandler(ctx *gin.Context) {
	if err := c.TestAllKeys(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "all keys tested"})
}

func (c *Coordinator) testPublicKeyHandler(ctx *gin.Context) {
	if err := c.TestPublicKeyOnly(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "public key tested"})
}

func (c *Coordinator) testRelinearizationKeyHandler(ctx *gin.Context) {
	if err := c.TestRelinearizationKeyOnly(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "relinearization key tested"})
}

func (c *Coordinator) testGaloisKeysHandler(ctx *gin.Context) {
	if err := c.TestGaloisKeysOnly(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "galois keys tested"})
}

func (c *Coordinator) getRelinearizationKeyStatusHandler(ctx *gin.Context) {
	status := c.GetStatus()
	ctx.JSON(http.StatusOK, status)
}
