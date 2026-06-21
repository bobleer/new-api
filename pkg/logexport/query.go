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

func eventPriority(eventType string) int {
	switch eventType {
	case EventTypeSessionTurn:
		return 3
	case EventTypeErrorLog:
		return 2
	case EventTypeConsumeLog:
		return 1
	default:
		return 0
	}
}

func turnFromEvent(traceID string, event Event) SessionTraceTurnQueryView {
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
	return turn
}

func mergeTurnView(existing, incoming SessionTraceTurnQueryView) SessionTraceTurnQueryView {
	if incoming.ErrorMessage != "" {
		existing.ErrorMessage = incoming.ErrorMessage
	}
	if incoming.Status == "error" {
		existing.Status = incoming.Status
	}
	if incoming.PromptTokens > existing.PromptTokens {
		existing.PromptTokens = incoming.PromptTokens
	}
	if incoming.CompletionTokens > existing.CompletionTokens {
		existing.CompletionTokens = incoming.CompletionTokens
	}
	if incoming.Detail != nil {
		if existing.Detail == nil {
			existing.Detail = incoming.Detail
		} else {
			if incoming.Detail.ClientRequest != "" {
				existing.Detail.ClientRequest = incoming.Detail.ClientRequest
			}
			if incoming.Detail.AssistantResponse != "" {
				existing.Detail.AssistantResponse = incoming.Detail.AssistantResponse
			}
		}
	}
	return existing
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

	hasSessionTurns := false
	for _, event := range events {
		if event.EventType == EventTypeSessionTurn {
			hasSessionTurns = true
			break
		}
	}

	view := &SessionTraceQueryView{TraceID: traceID, DataSource: source}
	turnByIndex := map[int]SessionTraceTurnQueryView{}
	turnPriority := map[int]int{}

	for _, event := range events {
		if event.TurnIndex <= 0 {
			continue
		}

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

		if hasSessionTurns {
			switch event.EventType {
			case EventTypeSessionTurn:
				turnByIndex[event.TurnIndex] = turnFromEvent(traceID, event)
				turnPriority[event.TurnIndex] = eventPriority(event.EventType)
			case EventTypeErrorLog:
				if existing, ok := turnByIndex[event.TurnIndex]; ok {
					if event.ErrorMessage != "" {
						existing.ErrorMessage = event.ErrorMessage
					}
					existing.Status = "error"
					turnByIndex[event.TurnIndex] = existing
				}
			}
			continue
		}

		priority := eventPriority(event.EventType)
		incoming := turnFromEvent(traceID, event)
		if existingPriority, ok := turnPriority[event.TurnIndex]; ok {
			if priority < existingPriority {
				continue
			}
			if priority == existingPriority {
				turnByIndex[event.TurnIndex] = mergeTurnView(turnByIndex[event.TurnIndex], incoming)
				continue
			}
		}
		turnByIndex[event.TurnIndex] = incoming
		turnPriority[event.TurnIndex] = priority
	}

	turnIndexes := make([]int, 0, len(turnByIndex))
	for turnIndex := range turnByIndex {
		turnIndexes = append(turnIndexes, turnIndex)
	}
	sort.Ints(turnIndexes)

	view.Turns = make([]SessionTraceTurnQueryView, 0, len(turnIndexes))
	for _, turnIndex := range turnIndexes {
		view.Turns = append(view.Turns, turnByIndex[turnIndex])
		if turnIndex > view.TurnCount {
			view.TurnCount = turnIndex
		}
	}
	if view.TurnCount == 0 {
		view.TurnCount = len(view.Turns)
	}
	return view
}

func GetSessionTraceTurnDetail(traceID string, turnIndex int) (*SessionTraceTurnDetailView, error) {
	view, err := QuerySessionTrace(traceID)
	if err != nil {
		return nil, err
	}
	for _, turn := range view.Turns {
		if turn.TurnIndex != turnIndex || turn.Detail == nil {
			continue
		}
		return turn.Detail, nil
	}
	return nil, fmt.Errorf("session trace turn detail not found")
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
