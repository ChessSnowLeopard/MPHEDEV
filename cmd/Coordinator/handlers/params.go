package handlers

import (
	"MPHEDev/cmd/Coordinator/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetCKKSParamsHandler(coordinator *services.Coordinator) gin.HandlerFunc {
	return func(c *gin.Context) {
		params, crp, galEls, galoisCRPs, rlkCRP := coordinator.GetParams()
		c.JSON(http.StatusOK, gin.H{
			"params":      params,
			"crp":         crp,
			"gal_els":     galEls,
			"galois_crps": galoisCRPs,
			"rlk_crp":     rlkCRP,
		})
	}
}
