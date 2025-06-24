package handlers

import (
	"MPHEDev/cmd/Coordinator/services"
	"MPHEDev/cmd/Coordinator/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

type GaloisKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	GalEl         uint64 `json:"gal_el"`
	ShareData     string `json:"share_data"`
}

type RelinearizationKeyShare struct {
	ParticipantID int    `json:"participant_id"`
	Round         int    `json:"round"`
	ShareData     string `json:"share_data"`
}

func PostPublicKeyHandler(coordinator *services.Coordinator) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req utils.PublicKeyShare
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		data, err := utils.DecodeFromBase64(req.ShareData)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64"})
			return
		}

		if err := coordinator.AddPublicKeyShare(req.ParticipantID, data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "received"})
	}
}

func PostSecretKeyHandler(coordinator *services.Coordinator) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req utils.SecretKeyShare
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		data, err := utils.DecodeFromBase64(req.ShareData)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64"})
			return
		}

		if err := coordinator.AddSecretKey(req.ParticipantID, data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "received"})
	}
}

func PostGaloisKeyHandler(coordinator *services.Coordinator) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req GaloisKeyShare
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		data, err := utils.DecodeFromBase64(req.ShareData)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64"})
			return
		}

		if err := coordinator.AddGaloisKeyShare(req.ParticipantID, req.GalEl, data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "received"})
	}
}

func PostRelinearizationKeyHandler(coordinator *services.Coordinator) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RelinearizationKeyShare
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		data, err := utils.DecodeFromBase64(req.ShareData)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64"})
			return
		}

		if err := coordinator.AddRelinearizationKeyShare(req.ParticipantID, req.Round, data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "received"})
	}
}

func GetRelinearizationKeyRound1AggregatedHandler(coordinator *services.Coordinator) gin.HandlerFunc {
	return func(c *gin.Context) {
		shareData, err := coordinator.GetRelinearizationKeyRound1Aggregated()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"share_data": shareData,
		})
	}
}
