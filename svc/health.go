package svc

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vincejv/gpon-parser/device"
)

func Health(c *gin.Context) {
	if !device.SvcHealth.GetFlag() {
		c.JSON(http.StatusServiceUnavailable, "Service is DOWN!")
		return
	}
	c.JSON(http.StatusOK, "Service is UP!")
}
