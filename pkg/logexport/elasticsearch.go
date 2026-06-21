package logexport

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/tidwall/gjson"
)

func exportToElasticsearch(event *Event) error {
	setting := currentSetting()
	if setting == nil || !elasticsearchConfigured() || event == nil {
		return nil
	}
	payload, err := common.Marshal(event)
	if err != nil {
		return err
	}
	index := elasticsearchIndexName(setting)
	action := fmt.Sprintf(`{"index":{"_index":"%s"}}`, index)
	body := action + "\n" + string(payload) + "\n"
	url := trimTrailingSlash(setting.ElasticsearchURL) + "/_bulk"
	respBody, statusCode, err := doRequest(http.MethodPost, url, []byte(body), map[string]string{
		"Content-Type": "application/x-ndjson",
	}, setting, true)
	if err != nil {
		return err
	}
	if statusCode >= http.StatusBadRequest {
		return fmt.Errorf("elasticsearch bulk export failed with status %d", statusCode)
	}
	parsed := gjson.ParseBytes(respBody)
	if parsed.Get("errors").Bool() {
		firstError := parsed.Get("items.0.index.error.reason").String()
		if firstError == "" {
			firstError = "unknown bulk error"
		}
		return fmt.Errorf("elasticsearch bulk export failed: %s", firstError)
	}
	return nil
}

func searchElasticsearchByTraceID(traceID string) ([]Event, error) {
	setting := currentSetting()
	if setting == nil || !elasticsearchConfigured() {
		return nil, fmt.Errorf("elasticsearch is not configured")
	}
	query := map[string]any{
		"size": 1000,
		"sort": []map[string]any{
			{"turn_index": map[string]string{"order": "asc", "missing": "_last"}},
			{"created_at": map[string]string{"order": "asc"}},
		},
		"query": map[string]any{
			"bool": map[string]any{
				"should": []map[string]any{
					{"term": map[string]string{"trace_id.keyword": traceID}},
					{"term": map[string]string{"trace_id": traceID}},
				},
				"minimum_should_match": 1,
			},
		},
	}
	queryBytes, err := common.Marshal(query)
	if err != nil {
		return nil, err
	}
	index := elasticsearchIndexName(setting)
	url := trimTrailingSlash(setting.ElasticsearchURL) + "/" + index + "/_search"
	respBody, _, err := doRequest(http.MethodPost, url, queryBytes, map[string]string{
		"Content-Type": "application/json",
	}, setting, true)
	if err != nil {
		return nil, err
	}
	parsed := gjson.ParseBytes(respBody)
	hits := parsed.Get("hits.hits").Array()
	if len(hits) == 0 {
		return nil, nil
	}
	events := make([]Event, 0, len(hits))
	for _, hit := range hits {
		source := hit.Get("_source").Raw
		if strings.TrimSpace(source) == "" {
			continue
		}
		var event Event
		if err := common.UnmarshalJsonStr(source, &event); err != nil {
			continue
		}
		events = append(events, event)
	}
	return events, nil
}

func testElasticsearchConnection() error {
	setting := currentSetting()
	if setting == nil || !elasticsearchConfigured() {
		return fmt.Errorf("elasticsearch is not configured")
	}
	url := trimTrailingSlash(setting.ElasticsearchURL)
	_, _, err := doRequest(http.MethodGet, url, nil, nil, setting, true)
	return err
}

func elasticsearchStatus() map[string]any {
	setting := currentSetting()
	status := map[string]any{
		"enabled": elasticsearchConfigured(),
	}
	if setting != nil {
		status["url"] = setting.ElasticsearchURL
		status["index"] = elasticsearchIndexName(setting)
	}
	if elasticsearchConfigured() {
		status["healthy"] = testElasticsearchConnection() == nil
	}
	return status
}
