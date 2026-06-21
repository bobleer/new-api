package logexport

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	EventTypeConsumeLog  = "consume_log"
	EventTypeErrorLog    = "error_log"
	EventTypeSessionTurn = "session_turn"
)

type Event struct {
	EventType         string `json:"event_type"`
	TraceID           string `json:"trace_id,omitempty"`
	RequestID         string `json:"request_id,omitempty"`
	UpstreamRequestID string `json:"upstream_request_id,omitempty"`
	LogID             int    `json:"log_id,omitempty"`
	TurnIndex         int    `json:"turn_index,omitempty"`
	UserID            int    `json:"user_id,omitempty"`
	Username          string `json:"username,omitempty"`
	TokenID           int    `json:"token_id,omitempty"`
	TokenName         string `json:"token_name,omitempty"`
	ModelName         string `json:"model_name,omitempty"`
	ChannelID         int    `json:"channel_id,omitempty"`
	Status            string `json:"status,omitempty"`
	LogType           int    `json:"log_type,omitempty"`
	PromptTokens      int    `json:"prompt_tokens,omitempty"`
	CompletionTokens  int    `json:"completion_tokens,omitempty"`
	Quota             int    `json:"quota,omitempty"`
	UseTimeSeconds    int    `json:"use_time_seconds,omitempty"`
	IsStream          bool   `json:"is_stream,omitempty"`
	Group             string `json:"group,omitempty"`
	Content           string `json:"content,omitempty"`
	ErrorMessage      string `json:"error_message,omitempty"`
	ClientRequest     string `json:"client_request,omitempty"`
	AssistantResponse string `json:"assistant_response,omitempty"`
	Other             string `json:"other,omitempty"`
	CreatedAt         int64  `json:"created_at"`
}

func TraceIDFromOther(other string) string {
	if other == "" {
		return ""
	}
	otherMap, err := common.StrToMap(other)
	if err != nil || otherMap == nil {
		return ""
	}
	if traceID, ok := otherMap["trace_id"].(string); ok {
		return strings.TrimSpace(traceID)
	}
	return ""
}
