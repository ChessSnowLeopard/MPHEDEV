package handlers

import (
	"MPHEDev/cmd/Coordinator/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetParticipantsHandler(coordinator *services.Coordinator) gin.HandlerFunc {
	return func(c *gin.Context) {
		participants := coordinator.GetParticipants()
		c.JSON(http.StatusOK, participants)
	}
}

func GetSetupStatusHandler(coordinator *services.Coordinator) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := coordinator.GetStatus()
		c.JSON(http.StatusOK, status)
	}
}
