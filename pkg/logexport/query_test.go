package logexport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withTestSettings(t *testing.T, fn func()) {
	t.Helper()
	backup := operation_setting.LogExportSetting{
		Enabled:              operation_setting.GetLogExportSetting().Enabled,
		ExportConsumeLogs:    operation_setting.GetLogExportSetting().ExportConsumeLogs,
		ExportErrorLogs:      operation_setting.GetLogExportSetting().ExportErrorLogs,
		ExportSessionTurns:   operation_setting.GetLogExportSetting().ExportSessionTurns,
		ElasticsearchEnabled: operation_setting.GetLogExportSetting().ElasticsearchEnabled,
		ElasticsearchURL:     operation_setting.GetLogExportSetting().ElasticsearchURL,
		ElasticsearchIndex:   operation_setting.GetLogExportSetting().ElasticsearchIndex,
		ClickHouseEnabled:    operation_setting.GetLogExportSetting().ClickHouseEnabled,
		ClickHouseURL:        operation_setting.GetLogExportSetting().ClickHouseURL,
		ClickHouseDatabase:   operation_setting.GetLogExportSetting().ClickHouseDatabase,
		ClickHouseTable:      operation_setting.GetLogExportSetting().ClickHouseTable,
	}
	t.Cleanup(func() {
		operation_setting.SetLogExportSettingForTest(backup)
	})
	fn()
}

func TestTraceIDFromOther(t *testing.T) {
	traceID := TraceIDFromOther(`{"trace_id":"11111111-1111-1111-1111-111111111111"}`)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", traceID)
}

func TestSearchElasticsearchByTraceID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/new-api-logs/_search":
			_, _ = w.Write([]byte(`{"hits":{"hits":[{"_source":{"event_type":"session_turn","trace_id":"11111111-1111-1111-1111-111111111111","turn_index":1,"created_at":100,"status":"success","client_request":"hello","assistant_response":"hi"}}]}}`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	withTestSettings(t, func() {
		operation_setting.SetLogExportSettingForTest(operation_setting.LogExportSetting{
			Enabled:              true,
			ExportSessionTurns:   true,
			ElasticsearchEnabled: true,
			ElasticsearchURL:     server.URL,
			ElasticsearchIndex:   "new-api-logs",
		})

		events, err := searchElasticsearchByTraceID("11111111-1111-1111-1111-111111111111")
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, EventTypeSessionTurn, events[0].EventType)
		assert.Equal(t, "hello", events[0].ClientRequest)
	})
}

func TestSearchClickHouseByTraceID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") == "SELECT 1" {
			_, _ = w.Write([]byte("1\n"))
			return
		}
		_, _ = w.Write([]byte(`{"event_type":"session_turn","trace_id":"11111111-1111-1111-1111-111111111111","turn_index":1,"created_at":100,"status":"success","client_request":"hello","assistant_response":"hi","is_stream":0}` + "\n"))
	}))
	defer server.Close()

	withTestSettings(t, func() {
		operation_setting.SetLogExportSettingForTest(operation_setting.LogExportSetting{
			Enabled:            true,
			ExportSessionTurns: true,
			ClickHouseEnabled:  true,
			ClickHouseURL:      server.URL,
			ClickHouseDatabase: "default",
			ClickHouseTable:    "new_api_log_events",
		})

		events, err := searchClickHouseByTraceID("11111111-1111-1111-1111-111111111111")
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, "hi", events[0].AssistantResponse)
	})
}

func TestBuildSessionTraceView(t *testing.T) {
	view := buildSessionTraceView("11111111-1111-1111-1111-111111111111", []Event{
		{
			EventType:         EventTypeSessionTurn,
			TraceID:           "11111111-1111-1111-1111-111111111111",
			TurnIndex:         1,
			Status:            "success",
			ClientRequest:     "req",
			AssistantResponse: "resp",
			CreatedAt:         100,
		},
	}, "clickhouse")
	require.NotNil(t, view)
	assert.Equal(t, "clickhouse", view.DataSource)
	require.Len(t, view.Turns, 1)
	assert.Equal(t, "req", view.Turns[0].Detail.ClientRequest)
}

func TestBuildSessionTraceViewDedupesExportEvents(t *testing.T) {
	traceID := "11111111-1111-1111-1111-111111111111"
	view := buildSessionTraceView(traceID, []Event{
		{
			EventType:     EventTypeConsumeLog,
			TraceID:       traceID,
			TurnIndex:     1,
			Status:        "success",
			CreatedAt:     100,
		},
		{
			EventType:         EventTypeSessionTurn,
			TraceID:           traceID,
			TurnIndex:         1,
			Status:            "success",
			ClientRequest:     "req",
			AssistantResponse: "resp",
			CreatedAt:         100,
		},
		{
			EventType:     EventTypeErrorLog,
			TraceID:       traceID,
			TurnIndex:     1,
			Status:        "error",
			ErrorMessage:  "failed",
			CreatedAt:     100,
		},
	}, "clickhouse")
	require.NotNil(t, view)
	require.Len(t, view.Turns, 1)
	assert.Equal(t, "req", view.Turns[0].Detail.ClientRequest)
	assert.Equal(t, "error", view.Turns[0].Status)
	assert.Equal(t, "failed", view.Turns[0].ErrorMessage)
}
