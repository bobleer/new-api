package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/logexport"

	"github.com/gin-gonic/gin"
)

func ExportConsumeLogFromParams(c *gin.Context, userID int, params model.RecordConsumeLogParams, username, requestID, upstreamRequestID string) {
	traceID := logexport.TraceIDFromOther(common.MapToJsonStr(params.Other))
	if traceID == "" && c != nil {
		traceID = common.GetContextKeyString(c, constant.ContextKeyTraceId)
	}
	logexport.ExportEvent(&logexport.Event{
		EventType:         logexport.EventTypeConsumeLog,
		TraceID:           traceID,
		RequestID:         requestID,
		UpstreamRequestID: upstreamRequestID,
		UserID:            userID,
		Username:          username,
		TokenID:           params.TokenId,
		TokenName:         params.TokenName,
		ModelName:         params.ModelName,
		ChannelID:         params.ChannelId,
		LogType:           model.LogTypeConsume,
		PromptTokens:      params.PromptTokens,
		CompletionTokens:  params.CompletionTokens,
		Quota:             params.Quota,
		UseTimeSeconds:    params.UseTimeSeconds,
		IsStream:          params.IsStream,
		Group:             params.Group,
		Content:           params.Content,
		Other:             common.MapToJsonStr(params.Other),
		Status:            "success",
		CreatedAt:         common.GetTimestamp(),
	})
}

func ExportConsumeLogEvent(c *gin.Context, log *model.Log, params model.RecordConsumeLogParams) {
	if log == nil {
		return
	}
	logexport.ExportEvent(&logexport.Event{
		EventType:         logexport.EventTypeConsumeLog,
		TraceID:           logexport.TraceIDFromOther(log.Other),
		RequestID:         log.RequestId,
		UpstreamRequestID: log.UpstreamRequestId,
		LogID:             log.Id,
		UserID:            log.UserId,
		Username:          log.Username,
		TokenID:           log.TokenId,
		TokenName:         log.TokenName,
		ModelName:         log.ModelName,
		ChannelID:         log.ChannelId,
		LogType:           log.Type,
		PromptTokens:      log.PromptTokens,
		CompletionTokens:  log.CompletionTokens,
		Quota:             log.Quota,
		UseTimeSeconds:    log.UseTime,
		IsStream:          log.IsStream,
		Group:             log.Group,
		Content:           log.Content,
		Other:             log.Other,
		Status:            "success",
		CreatedAt:         log.CreatedAt,
	})
}

func ExportErrorLogEvent(c *gin.Context, log *model.Log, content string) {
	if log == nil {
		return
	}
	traceID := logexport.TraceIDFromOther(log.Other)
	if traceID == "" && c != nil {
		traceID = common.GetContextKeyString(c, constant.ContextKeyTraceId)
	}
	logexport.ExportEvent(&logexport.Event{
		EventType:         logexport.EventTypeErrorLog,
		TraceID:           traceID,
		RequestID:         log.RequestId,
		UpstreamRequestID: log.UpstreamRequestId,
		LogID:             log.Id,
		UserID:            log.UserId,
		Username:          log.Username,
		TokenID:           log.TokenId,
		TokenName:         log.TokenName,
		ModelName:         log.ModelName,
		ChannelID:         log.ChannelId,
		LogType:           log.Type,
		UseTimeSeconds:    log.UseTime,
		IsStream:          log.IsStream,
		Group:             log.Group,
		Content:           content,
		ErrorMessage:      content,
		Other:             log.Other,
		Status:            "error",
		CreatedAt:         log.CreatedAt,
	})
}

func ExportErrorLogEventWithDetail(c *gin.Context, log *model.Log, content, clientRequest, upstreamResponse string) {
	traceID := ""
	requestID := ""
	upstreamRequestID := ""
	userID := 0
	tokenID := 0
	modelName := ""
	channelID := 0
	isStream := false
	group := ""
	other := ""
	createdAt := common.GetTimestamp()
	if log != nil {
		traceID = logexport.TraceIDFromOther(log.Other)
		requestID = log.RequestId
		upstreamRequestID = log.UpstreamRequestId
		userID = log.UserId
		tokenID = log.TokenId
		modelName = log.ModelName
		channelID = log.ChannelId
		isStream = log.IsStream
		group = log.Group
		other = log.Other
		createdAt = log.CreatedAt
	}
	if c != nil {
		if traceID == "" {
			traceID = common.GetContextKeyString(c, constant.ContextKeyTraceId)
		}
		if requestID == "" {
			requestID = c.GetString(common.RequestIdKey)
		}
		if upstreamRequestID == "" {
			upstreamRequestID = c.GetString(common.UpstreamRequestIdKey)
		}
		if userID == 0 {
			userID = c.GetInt("id")
		}
		if tokenID == 0 {
			tokenID = c.GetInt("token_id")
		}
		if modelName == "" {
			modelName = c.GetString("original_model")
		}
		if channelID == 0 {
			channelID = c.GetInt("channel_id")
		}
		if !isStream {
			isStream = common.GetContextKeyBool(c, constant.ContextKeyIsStream)
		}
		if group == "" {
			group = c.GetString("group")
		}
	}
	logexport.ExportEvent(&logexport.Event{
		EventType:         logexport.EventTypeErrorLog,
		TraceID:           traceID,
		RequestID:         requestID,
		UpstreamRequestID: upstreamRequestID,
		UserID:            userID,
		TokenID:           tokenID,
		ModelName:         modelName,
		ChannelID:         channelID,
		LogType:           model.LogTypeError,
		IsStream:          isStream,
		Group:             group,
		Content:           content,
		ErrorMessage:      content,
		ClientRequest:     clientRequest,
		AssistantResponse: upstreamResponse,
		Other:             other,
		Status:            "error",
		CreatedAt:         createdAt,
	})
}
