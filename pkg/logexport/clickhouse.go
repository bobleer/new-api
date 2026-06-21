package logexport

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/tidwall/gjson"
)

const clickHouseCreateTableSQL = `
CREATE TABLE IF NOT EXISTS %s.%s
(
	event_type String,
	trace_id String,
	request_id String,
	upstream_request_id String,
	log_id Int64,
	turn_index Int32,
	user_id Int32,
	username String,
	token_id Int32,
	token_name String,
	model_name String,
	channel_id Int32,
	status String,
	log_type Int32,
	prompt_tokens Int32,
	completion_tokens Int32,
	quota Int64,
	use_time_seconds Int32,
	is_stream UInt8,
	group_name String,
	content String,
	error_message String,
	client_request String,
	assistant_response String,
	other String,
	created_at DateTime64(3, 'UTC')
)
ENGINE = MergeTree
ORDER BY (trace_id, created_at, request_id)
`

func exportToClickHouse(event *Event) error {
	setting := currentSetting()
	if setting == nil || !clickHouseConfigured() || event == nil {
		return nil
	}
	if err := ensureClickHouseTable(); err != nil {
		return err
	}
	row := clickHouseRowFromEvent(event)
	payload, err := common.Marshal(row)
	if err != nil {
		return err
	}
	query := fmt.Sprintf("INSERT INTO %s FORMAT JSONEachRow", clickHouseQualifiedTable(setting))
	endpoint := trimTrailingSlash(setting.ClickHouseURL) + "/?" + url.Values{"query": {query}}.Encode()
	_, _, err = doRequest(http.MethodPost, endpoint, append(payload, '\n'), map[string]string{
		"Content-Type": "application/json",
	}, setting, false)
	return err
}

type clickHouseRow struct {
	EventType         string `json:"event_type"`
	TraceID           string `json:"trace_id"`
	RequestID         string `json:"request_id"`
	UpstreamRequestID string `json:"upstream_request_id"`
	LogID             int64  `json:"log_id"`
	TurnIndex         int32  `json:"turn_index"`
	UserID            int32  `json:"user_id"`
	Username          string `json:"username"`
	TokenID           int32  `json:"token_id"`
	TokenName         string `json:"token_name"`
	ModelName         string `json:"model_name"`
	ChannelID         int32  `json:"channel_id"`
	Status            string `json:"status"`
	LogType           int32  `json:"log_type"`
	PromptTokens      int32  `json:"prompt_tokens"`
	CompletionTokens  int32  `json:"completion_tokens"`
	Quota             int64  `json:"quota"`
	UseTimeSeconds    int32  `json:"use_time_seconds"`
	IsStream          uint8  `json:"is_stream"`
	GroupName         string `json:"group_name"`
	Content           string `json:"content"`
	ErrorMessage      string `json:"error_message"`
	ClientRequest     string `json:"client_request"`
	AssistantResponse string `json:"assistant_response"`
	Other             string `json:"other"`
	CreatedAt         string `json:"created_at"`
}

func clickHouseRowFromEvent(event *Event) clickHouseRow {
	isStream := uint8(0)
	if event.IsStream {
		isStream = 1
	}
	return clickHouseRow{
		EventType:         event.EventType,
		TraceID:           event.TraceID,
		RequestID:         event.RequestID,
		UpstreamRequestID: event.UpstreamRequestID,
		LogID:             int64(event.LogID),
		TurnIndex:         int32(event.TurnIndex),
		UserID:            int32(event.UserID),
		Username:          event.Username,
		TokenID:           int32(event.TokenID),
		TokenName:         event.TokenName,
		ModelName:         event.ModelName,
		ChannelID:         int32(event.ChannelID),
		Status:            event.Status,
		LogType:           int32(event.LogType),
		PromptTokens:      int32(event.PromptTokens),
		CompletionTokens:  int32(event.CompletionTokens),
		Quota:             int64(event.Quota),
		UseTimeSeconds:    int32(event.UseTimeSeconds),
		IsStream:          isStream,
		GroupName:         event.Group,
		Content:           event.Content,
		ErrorMessage:      event.ErrorMessage,
		ClientRequest:     event.ClientRequest,
		AssistantResponse: event.AssistantResponse,
		Other:             event.Other,
		CreatedAt:         formatClickHouseDateTime(event.CreatedAt),
	}
}

func formatClickHouseDateTime(unixSeconds int64) string {
	if unixSeconds <= 0 {
		unixSeconds = common.GetTimestamp()
	}
	return time.Unix(unixSeconds, 0).UTC().Format("2006-01-02 15:04:05.000")
}

func ensureClickHouseTable() error {
	setting := currentSetting()
	if setting == nil || !clickHouseConfigured() {
		return nil
	}
	query := fmt.Sprintf(
		clickHouseCreateTableSQL,
		clickHouseDatabaseName(setting),
		clickHouseTableName(setting),
	)
	endpoint := trimTrailingSlash(setting.ClickHouseURL) + "/?" + url.Values{"query": {query}}.Encode()
	_, _, err := doRequest(http.MethodPost, endpoint, nil, nil, setting, false)
	return err
}

func searchClickHouseByTraceID(traceID string) ([]Event, error) {
	setting := currentSetting()
	if setting == nil || !clickHouseConfigured() {
		return nil, fmt.Errorf("clickhouse is not configured")
	}
	query := fmt.Sprintf(
		"SELECT event_type, trace_id, request_id, upstream_request_id, log_id, turn_index, user_id, username, token_id, token_name, model_name, channel_id, status, log_type, prompt_tokens, completion_tokens, quota, use_time_seconds, is_stream, group_name, content, error_message, client_request, assistant_response, other, toUnixTimestamp(created_at) AS created_at FROM %s WHERE trace_id = {trace_id:String} ORDER BY turn_index ASC, created_at ASC LIMIT 1000 FORMAT JSONEachRow",
		clickHouseQualifiedTable(setting),
	)
	endpoint := trimTrailingSlash(setting.ClickHouseURL) + "/?" + url.Values{
		"query":         {query},
		"param_trace_id": {traceID},
	}.Encode()
	respBody, _, err := doRequest(http.MethodPost, endpoint, nil, nil, setting, false)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(respBody)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil, nil
	}
	events := make([]Event, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parsed := gjson.Parse(line)
		event := Event{
			EventType:         parsed.Get("event_type").String(),
			TraceID:           parsed.Get("trace_id").String(),
			RequestID:         parsed.Get("request_id").String(),
			UpstreamRequestID: parsed.Get("upstream_request_id").String(),
			LogID:             int(parsed.Get("log_id").Int()),
			TurnIndex:         int(parsed.Get("turn_index").Int()),
			UserID:            int(parsed.Get("user_id").Int()),
			Username:          parsed.Get("username").String(),
			TokenID:           int(parsed.Get("token_id").Int()),
			TokenName:         parsed.Get("token_name").String(),
			ModelName:         parsed.Get("model_name").String(),
			ChannelID:         int(parsed.Get("channel_id").Int()),
			Status:            parsed.Get("status").String(),
			LogType:           int(parsed.Get("log_type").Int()),
			PromptTokens:      int(parsed.Get("prompt_tokens").Int()),
			CompletionTokens:  int(parsed.Get("completion_tokens").Int()),
			Quota:             int(parsed.Get("quota").Int()),
			UseTimeSeconds:    int(parsed.Get("use_time_seconds").Int()),
			IsStream:          parsed.Get("is_stream").Uint() == 1,
			Group:             parsed.Get("group_name").String(),
			Content:           parsed.Get("content").String(),
			ErrorMessage:      parsed.Get("error_message").String(),
			ClientRequest:     parsed.Get("client_request").String(),
			AssistantResponse: parsed.Get("assistant_response").String(),
			Other:             parsed.Get("other").String(),
			CreatedAt:         parsed.Get("created_at").Int(),
		}
		events = append(events, event)
	}
	return events, nil
}

func testClickHouseConnection() error {
	setting := currentSetting()
	if setting == nil || !clickHouseConfigured() {
		return fmt.Errorf("clickhouse is not configured")
	}
	endpoint := trimTrailingSlash(setting.ClickHouseURL) + "/?query=SELECT+1"
	_, _, err := doRequest(http.MethodGet, endpoint, nil, nil, setting, false)
	return err
}

func clickHouseStatus() map[string]any {
	setting := currentSetting()
	status := map[string]any{
		"enabled": clickHouseConfigured(),
	}
	if setting != nil {
		status["url"] = setting.ClickHouseURL
		status["database"] = clickHouseDatabaseName(setting)
		status["table"] = clickHouseTableName(setting)
	}
	if clickHouseConfigured() {
		status["healthy"] = testClickHouseConnection() == nil
	}
	return status
}
