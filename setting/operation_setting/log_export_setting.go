package operation_setting

import (
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type LogExportSetting struct {
	Enabled                     bool   `json:"enabled"`
	ExportConsumeLogs           bool   `json:"export_consume_logs"`
	ExportErrorLogs             bool   `json:"export_error_logs"`
	ExportSessionTurns          bool   `json:"export_session_turns"`
	ElasticsearchEnabled        bool   `json:"elasticsearch_enabled"`
	ElasticsearchURL            string `json:"elasticsearch_url"`
	ElasticsearchIndex          string `json:"elasticsearch_index"`
	ElasticsearchUsername       string `json:"elasticsearch_username"`
	ElasticsearchSecret         string `json:"elasticsearch_secret"`
	ElasticsearchAPIKey         string `json:"elasticsearch_api_key"`
	ClickHouseEnabled           bool   `json:"clickhouse_enabled"`
	ClickHouseURL               string `json:"clickhouse_url"`
	ClickHouseDatabase          string `json:"clickhouse_database"`
	ClickHouseTable             string `json:"clickhouse_table"`
	ClickHouseUsername          string `json:"clickhouse_username"`
	ClickHouseSecret            string `json:"clickhouse_secret"`
	PreferExternalForTraceQuery bool   `json:"prefer_external_for_trace_query"`
}

var logExportSetting = LogExportSetting{
	ExportConsumeLogs:  true,
	ExportErrorLogs:    true,
	ExportSessionTurns: true,
	ElasticsearchIndex: "new-api-logs",
	ClickHouseDatabase: "default",
	ClickHouseTable:    "new_api_log_events",
}

func init() {
	config.GlobalConfig.Register("log_export_setting", &logExportSetting)
}

func GetLogExportSetting() *LogExportSetting {
	setting := logExportSetting
	if v := strings.TrimSpace(os.Getenv("LOG_EXPORT_ENABLED")); v != "" {
		setting.Enabled = common.GetEnvOrDefaultBool("LOG_EXPORT_ENABLED", setting.Enabled)
	}
	if common.GetEnvOrDefaultBool("LOG_EXPORT_ES_ENABLED", false) {
		setting.ElasticsearchEnabled = true
	}
	if url := strings.TrimSpace(os.Getenv("LOG_EXPORT_ES_URL")); url != "" {
		setting.ElasticsearchURL = url
	}
	if index := strings.TrimSpace(os.Getenv("LOG_EXPORT_ES_INDEX")); index != "" {
		setting.ElasticsearchIndex = index
	}
	if user := strings.TrimSpace(os.Getenv("LOG_EXPORT_ES_USERNAME")); user != "" {
		setting.ElasticsearchUsername = user
	}
	if secret := strings.TrimSpace(os.Getenv("LOG_EXPORT_ES_SECRET")); secret != "" {
		setting.ElasticsearchSecret = secret
	}
	if apiKey := strings.TrimSpace(os.Getenv("LOG_EXPORT_ES_API_KEY")); apiKey != "" {
		setting.ElasticsearchAPIKey = apiKey
	}
	if common.GetEnvOrDefaultBool("LOG_EXPORT_CH_ENABLED", false) {
		setting.ClickHouseEnabled = true
	}
	if url := strings.TrimSpace(os.Getenv("LOG_EXPORT_CH_URL")); url != "" {
		setting.ClickHouseURL = url
	}
	if db := strings.TrimSpace(os.Getenv("LOG_EXPORT_CH_DATABASE")); db != "" {
		setting.ClickHouseDatabase = db
	}
	if table := strings.TrimSpace(os.Getenv("LOG_EXPORT_CH_TABLE")); table != "" {
		setting.ClickHouseTable = table
	}
	if user := strings.TrimSpace(os.Getenv("LOG_EXPORT_CH_USERNAME")); user != "" {
		setting.ClickHouseUsername = user
	}
	if secret := strings.TrimSpace(os.Getenv("LOG_EXPORT_CH_SECRET")); secret != "" {
		setting.ClickHouseSecret = secret
	}
	if v := strings.TrimSpace(os.Getenv("LOG_EXPORT_PREFER_EXTERNAL_TRACE_QUERY")); v != "" {
		setting.PreferExternalForTraceQuery = common.GetEnvOrDefaultBool("LOG_EXPORT_PREFER_EXTERNAL_TRACE_QUERY", setting.PreferExternalForTraceQuery)
	}
	if setting.Enabled && (setting.ElasticsearchEnabled || setting.ClickHouseEnabled) {
		return &setting
	}
	if setting.Enabled && setting.ElasticsearchURL != "" {
		setting.ElasticsearchEnabled = true
	}
	if setting.Enabled && setting.ClickHouseURL != "" {
		setting.ClickHouseEnabled = true
	}
	return &setting
}

func SetLogExportSettingForTest(setting LogExportSetting) {
	logExportSetting = setting
}
