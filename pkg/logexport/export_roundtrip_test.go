package logexport

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportAndQueryRoundTripWithMockBackends(t *testing.T) {
	esHits := 0
	chHits := 0
	esServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		esHits++
		switch r.URL.Path {
		case "/_bulk":
			_, _ = w.Write([]byte(`{"errors":false,"items":[{"index":{"status":201}}]}`))
		case "/new-api-logs/_search":
			_, _ = w.Write([]byte(`{"hits":{"hits":[{"_source":{"event_type":"session_turn","trace_id":"22222222-2222-2222-2222-222222222222","turn_index":1,"created_at":100,"status":"success","client_request":"req","assistant_response":"resp"}}]}}`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer esServer.Close()

	chServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		chHits++
		if query == "SELECT 1" {
			_, _ = w.Write([]byte("1\n"))
			return
		}
		if strings.Contains(query, "CREATE TABLE") || strings.Contains(query, "INSERT INTO") {
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write([]byte(`{"event_type":"session_turn","trace_id":"33333333-3333-3333-3333-333333333333","turn_index":1,"created_at":100,"status":"success","client_request":"ch-req","assistant_response":"ch-resp","is_stream":0}` + "\n"))
	}))
	defer chServer.Close()

	withTestSettings(t, func() {
		operation_setting.SetLogExportSettingForTest(operation_setting.LogExportSetting{
			Enabled:              true,
			ExportSessionTurns:   true,
			ExportErrorLogs:      true,
			ExportConsumeLogs:    true,
			ElasticsearchEnabled: true,
			ElasticsearchURL:     esServer.URL,
			ElasticsearchIndex:   "new-api-logs",
			ClickHouseEnabled:    true,
			ClickHouseURL:        chServer.URL,
			ClickHouseDatabase:   "default",
			ClickHouseTable:      "new_api_log_events",
		})

		traceID := "22222222-2222-2222-2222-222222222222"
		event := &Event{
			EventType:         EventTypeSessionTurn,
			TraceID:           traceID,
			TurnIndex:         1,
			Status:            "success",
			ClientRequest:     "req",
			AssistantResponse: "resp",
			CreatedAt:         100,
		}
		require.NoError(t, exportToElasticsearch(event))
		require.NoError(t, exportToClickHouse(event))
		assert.Greater(t, esHits, 0)
		assert.Greater(t, chHits, 0)

		view, err := QuerySessionTrace("33333333-3333-3333-3333-333333333333")
		require.NoError(t, err)
		require.NotNil(t, view)
		assert.Equal(t, "clickhouse", view.DataSource)
		require.Len(t, view.Turns, 1)
		assert.Equal(t, "ch-req", view.Turns[0].Detail.ClientRequest)

		detail, err := GetSessionTraceTurnDetail("33333333-3333-3333-3333-333333333333", 1)
		require.NoError(t, err)
		assert.Equal(t, "ch-resp", detail.AssistantResponse)

		status := TestConnections()
		esStatus, ok := status["elasticsearch"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, esStatus["healthy"])
	})
}
