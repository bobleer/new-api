package logexport

import (
	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
)

func ExportEvent(event *Event) {
	if event == nil || !IsEnabled() {
		return
	}
	gopool.Go(func() {
		setting := currentSetting()
		if setting == nil {
			return
		}
		switch event.EventType {
		case EventTypeConsumeLog:
			if !setting.ExportConsumeLogs {
				return
			}
		case EventTypeErrorLog:
			if !setting.ExportErrorLogs {
				return
			}
		case EventTypeSessionTurn:
			if !setting.ExportSessionTurns {
				return
			}
		}
		if elasticsearchConfigured() {
			if err := exportToElasticsearch(event); err != nil {
				common.SysError("failed to export log event to elasticsearch: " + err.Error())
			}
		}
		if clickHouseConfigured() {
			if err := exportToClickHouse(event); err != nil {
				common.SysError("failed to export log event to clickhouse: " + err.Error())
			}
		}
	})
}

type SessionTurnExportParams struct {
	TraceID          string
	TurnIndex        int
	RequestID        string
	LogID            int
	UserID           int
	TokenID          int
	ModelName        string
	ChannelID        int
	Status           string
	PromptTokens     int
	CompletionTokens int
	IsStream         bool
	ErrorMessage     string
	ClientRequest    string
	AssistantResponse string
	CreatedAt        int64
}

func ExportSessionTurn(params SessionTurnExportParams) {
	ExportEvent(&Event{
		EventType:         EventTypeSessionTurn,
		TraceID:           params.TraceID,
		RequestID:         params.RequestID,
		LogID:             params.LogID,
		TurnIndex:         params.TurnIndex,
		UserID:            params.UserID,
		TokenID:           params.TokenID,
		ModelName:         params.ModelName,
		ChannelID:         params.ChannelID,
		Status:            params.Status,
		PromptTokens:      params.PromptTokens,
		CompletionTokens:  params.CompletionTokens,
		IsStream:          params.IsStream,
		ErrorMessage:      params.ErrorMessage,
		ClientRequest:     params.ClientRequest,
		AssistantResponse: params.AssistantResponse,
		CreatedAt:         params.CreatedAt,
	})
}
