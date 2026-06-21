package logexport

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const defaultHTTPTimeout = 10 * time.Second

func doRequest(method, rawURL string, body []byte, headers map[string]string, setting *operation_setting.LogExportSetting, useElasticsearch bool) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if useElasticsearch {
		applyElasticsearchAuth(req, setting)
	} else {
		applyClickHouseAuth(req, setting)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 300 {
		return respBody, resp.StatusCode, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, resp.StatusCode, nil
}

func applyElasticsearchAuth(req *http.Request, setting *operation_setting.LogExportSetting) {
	if setting == nil {
		return
	}
	if apiKey := strings.TrimSpace(setting.ElasticsearchAPIKey); apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+apiKey)
		return
	}
	if user := strings.TrimSpace(setting.ElasticsearchUsername); user != "" {
		secret := strings.TrimSpace(setting.ElasticsearchSecret)
		token := base64.StdEncoding.EncodeToString([]byte(user + ":" + secret))
		req.Header.Set("Authorization", "Basic "+token)
	}
}

func applyClickHouseAuth(req *http.Request, setting *operation_setting.LogExportSetting) {
	if setting == nil {
		return
	}
	if user := strings.TrimSpace(setting.ClickHouseUsername); user != "" {
		req.SetBasicAuth(user, strings.TrimSpace(setting.ClickHouseSecret))
	}
}

func trimTrailingSlash(url string) string {
	return strings.TrimRight(strings.TrimSpace(url), "/")
}

func elasticsearchIndexName(setting *operation_setting.LogExportSetting) string {
	if setting == nil || strings.TrimSpace(setting.ElasticsearchIndex) == "" {
		return "new-api-logs"
	}
	return strings.TrimSpace(setting.ElasticsearchIndex)
}

func clickHouseTableName(setting *operation_setting.LogExportSetting) string {
	if setting == nil || strings.TrimSpace(setting.ClickHouseTable) == "" {
		return "new_api_log_events"
	}
	return strings.TrimSpace(setting.ClickHouseTable)
}

func clickHouseDatabaseName(setting *operation_setting.LogExportSetting) string {
	if setting == nil || strings.TrimSpace(setting.ClickHouseDatabase) == "" {
		return "default"
	}
	return strings.TrimSpace(setting.ClickHouseDatabase)
}

func clickHouseQualifiedTable(setting *operation_setting.LogExportSetting) string {
	return clickHouseDatabaseName(setting) + "." + clickHouseTableName(setting)
}
