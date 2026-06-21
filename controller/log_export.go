package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/logexport"

	"github.com/gin-gonic/gin"
)

func GetLogExportStatus(c *gin.Context) {
	common.ApiSuccess(c, logexport.Status())
}

func TestLogExportConnections(c *gin.Context) {
	common.ApiSuccess(c, logexport.TestConnections())
}
