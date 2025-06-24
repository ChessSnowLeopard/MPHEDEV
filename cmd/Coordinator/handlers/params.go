package handlers

import (
	"MPHEDev/cmd/Coordinator/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetCKKSParamsHandler(coordinator *services.Coordinator) gin.HandlerFunc {
	return func(c *gin.Context) {
		params, crp := coordinator.GetParams()
		c.JSON(http.StatusOK, gin.H{
			"params": params,
			"crp":    crp,
		})
	}
}
