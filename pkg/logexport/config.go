package logexport

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func currentSetting() *operation_setting.LogExportSetting {
	return operation_setting.GetLogExportSetting()
}

func IsEnabled() bool {
	setting := currentSetting()
	if setting == nil {
		return false
	}
	return setting.Enabled && (setting.ElasticsearchEnabled || setting.ClickHouseEnabled)
}

func ShouldPreferExternalTraceQuery() bool {
	setting := currentSetting()
	if setting == nil {
		return false
	}
	return setting.PreferExternalForTraceQuery
}

func elasticsearchConfigured() bool {
	setting := currentSetting()
	return setting != nil && setting.Enabled && setting.ElasticsearchEnabled && strings.TrimSpace(setting.ElasticsearchURL) != ""
}

func clickHouseConfigured() bool {
	setting := currentSetting()
	return setting != nil && setting.Enabled && setting.ClickHouseEnabled && strings.TrimSpace(setting.ClickHouseURL) != ""
}
