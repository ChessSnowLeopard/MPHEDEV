package handlers

import (
	"MPHEDev/cmd/Coordinator/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegisterHandler(coordinator *services.Coordinator) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := coordinator.RegisterParticipant()
		c.JSON(http.StatusOK, gin.H{"participant_id": id})
	}
}
