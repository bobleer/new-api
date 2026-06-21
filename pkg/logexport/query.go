package logexport

import (
	"fmt"
	"sort"
)

type SessionTraceQueryView struct {
	TraceID        string                      `json:"trace_id"`
	UserID         int                         `json:"user_id"`
	TokenID        int                         `json:"token_id"`
	ModelName      string                      `json:"model_name"`
	TurnCount      int                         `json:"turn_count"`
	CreatedAt      int64                       `json:"created_at"`
	LastActivityAt int64                       `json:"last_activity_at"`
	Turns          []SessionTraceTurnQueryView `json:"turns"`
	DataSource     string                      `json:"data_source"`
}

type SessionTraceTurnQueryView struct {
	ID               int    `json:"id"`
	TurnIndex        int    `json:"turn_index"`
	RequestID        string `json:"request_id"`
	UserID           int    `json:"user_id"`
	TokenID          int    `json:"token_id"`
	ModelName        string `json:"model_name"`
	ChannelID        int    `json:"channel_id"`
	Status           string `json:"status"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	IsStream         bool   `json:"is_stream"`
	ErrorMessage     string `json:"error_message,omitempty"`
	CreatedAt        int64  `json:"created_at"`
	Detail           *SessionTraceTurnDetailView `json:"detail,omitempty"`
}

type SessionTraceTurnDetailView struct {
	TraceID           string `json:"trace_id"`
	TurnIndex         int    `json:"turn_index"`
	RequestID         string `json:"request_id,omitempty"`
	ClientRequest     string `json:"client_request,omitempty"`
	AssistantResponse string `json:"assistant_response,omitempty"`
	IsStream          bool   `json:"is_stream,omitempty"`
	Truncated         bool   `json:"truncated,omitempty"`
}

func QuerySessionTrace(traceID string) (*SessionTraceQueryView, error) {
	if !IsEnabled() {
		return nil, fmt.Errorf("log export is not enabled")
	}
	var (
		events []Event
		source string
	)
	if clickHouseConfigured() {
		chEvents, err := searchClickHouseByTraceID(traceID)
		if err == nil && len(chEvents) > 0 {
			events = chEvents
			source = "clickhouse"
		}
	}
	if len(events) == 0 && elasticsearchConfigured() {
		esEvents, err := searchElasticsearchByTraceID(traceID)
		if err == nil && len(esEvents) > 0 {
			events = esEvents
			source = "elasticsearch"
		}
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("no exported events found for trace id")
	}
	return buildSessionTraceView(traceID, events, source), nil
}

func buildSessionTraceView(traceID string, events []Event, source string) *SessionTraceQueryView {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].TurnIndex != events[j].TurnIndex {
			if events[i].TurnIndex == 0 {
				return false
			}
			if events[j].TurnIndex == 0 {
				return true
			}
			return events[i].TurnIndex < events[j].TurnIndex
		}
		return events[i].CreatedAt < events[j].CreatedAt
	})

	view := &SessionTraceQueryView{TraceID: traceID, DataSource: source}
	for _, event := range events {
		if view.UserID == 0 && event.UserID > 0 {
			view.UserID = event.UserID
		}
		if view.TokenID == 0 && event.TokenID > 0 {
			view.TokenID = event.TokenID
		}
		if view.ModelName == "" && event.ModelName != "" {
			view.ModelName = event.ModelName
		}
		if view.CreatedAt == 0 || (event.CreatedAt > 0 && event.CreatedAt < view.CreatedAt) {
			view.CreatedAt = event.CreatedAt
		}
		if event.CreatedAt > view.LastActivityAt {
			view.LastActivityAt = event.CreatedAt
		}
		if event.TurnIndex > view.TurnCount {
			view.TurnCount = event.TurnIndex
		}

		turn := SessionTraceTurnQueryView{
			ID:               event.LogID,
			TurnIndex:        event.TurnIndex,
			RequestID:        event.RequestID,
			UserID:           event.UserID,
			TokenID:          event.TokenID,
			ModelName:        event.ModelName,
			ChannelID:        event.ChannelID,
			Status:           event.Status,
			PromptTokens:     event.PromptTokens,
			CompletionTokens: event.CompletionTokens,
			IsStream:         event.IsStream,
			ErrorMessage:     event.ErrorMessage,
			CreatedAt:        event.CreatedAt,
		}
		if event.EventType == EventTypeSessionTurn || event.ClientRequest != "" || event.AssistantResponse != "" {
			turn.Detail = &SessionTraceTurnDetailView{
				TraceID:           traceID,
				TurnIndex:         event.TurnIndex,
				RequestID:         event.RequestID,
				ClientRequest:     event.ClientRequest,
				AssistantResponse: event.AssistantResponse,
				IsStream:          event.IsStream,
			}
		}
		if turn.Status == "" {
			if event.EventType == EventTypeErrorLog {
				turn.Status = "error"
			} else {
				turn.Status = "success"
			}
		}
		view.Turns = append(view.Turns, turn)
	}
	if view.TurnCount == 0 {
		view.TurnCount = len(view.Turns)
	}
	return view
}

func Status() map[string]any {
	setting := currentSetting()
	result := map[string]any{
		"enabled":                         IsEnabled(),
		"prefer_external_for_trace_query": setting != nil && setting.PreferExternalForTraceQuery,
		"elasticsearch":                   elasticsearchStatus(),
		"clickhouse":                      clickHouseStatus(),
	}
	return result
}

func TestConnections() map[string]any {
	result := map[string]any{
		"elasticsearch": map[string]any{
			"configured": elasticsearchConfigured(),
		},
		"clickhouse": map[string]any{
			"configured": clickHouseConfigured(),
		},
	}
	if elasticsearchConfigured() {
		err := testElasticsearchConnection()
		result["elasticsearch"] = map[string]any{
			"configured": true,
			"healthy":    err == nil,
			"message":    errorMessage(err),
		}
	}
	if clickHouseConfigured() {
		err := testClickHouseConnection()
		if err == nil {
			err = ensureClickHouseTable()
		}
		result["clickhouse"] = map[string]any{
			"configured": true,
			"healthy":    err == nil,
			"message":    errorMessage(err),
		}
	}
	return result
}

func errorMessage(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}
