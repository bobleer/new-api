package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetSessionTrace(c *gin.Context) {
	traceID := strings.TrimSpace(c.Param("trace_id"))
	if !model.IsValidTraceID(traceID) {
		common.ApiErrorMsg(c, "invalid trace id")
		return
	}

	view, err := service.GetSessionTraceFullView(traceID)
	if err != nil {
		common.ApiErrorMsg(c, "session trace not found")
		return
	}

	common.ApiSuccess(c, view)
}

func DownloadSessionTraceTurn(c *gin.Context) {
	traceID := strings.TrimSpace(c.Param("trace_id"))
	turnIndex := common.String2Int(c.Param("turn_index"))
	if !model.IsValidTraceID(traceID) || turnIndex <= 0 {
		common.ApiErrorMsg(c, "invalid trace id or turn index")
		return
	}

	detail, err := model.LoadSessionTraceTurnDetail(traceID, turnIndex)
	if err != nil {
		common.ApiErrorMsg(c, "session trace turn detail not found")
		return
	}

	data, err := common.Marshal(detail)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	filename := "session-trace-" + traceID + "-turn-" + c.Param("turn_index") + ".json"
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}
