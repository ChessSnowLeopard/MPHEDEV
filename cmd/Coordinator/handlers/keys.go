package handlers

import (
	"MPHEDev/cmd/Coordinator/services"
	"MPHEDev/cmd/Coordinator/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

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
