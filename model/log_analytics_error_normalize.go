package model

import (
	"regexp"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const logAnalyticsMaxErrorScanRows = 2000

var (
	logAnalyticsErrorUUIDPattern  = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	logAnalyticsErrorHexPattern   = regexp.MustCompile(`(?i)\b[0-9a-f]{16,}\b`)
	logAnalyticsErrorNumberPattern = regexp.MustCompile(`\b\d{2,}\b`)
)

type logAnalyticsErrorScanRow struct {
	Message   string `gorm:"column:message"`
	ModelName string `gorm:"column:model_name"`
	ChannelID int    `gorm:"column:channel_id"`
	CreatedAt int64  `gorm:"column:created_at"`
}

type logAnalyticsErrorClusterKey struct {
	normalizedKey string
	modelName     string
	channelID     int
}

type logAnalyticsErrorClusterAccumulator struct {
	count     int64
	latestAt  int64
	message   string
	modelName string
	channelID int
}

func normalizeLogAnalyticsErrorKey(message string) string {
	s := strings.TrimSpace(message)
	if s == "" {
		return "-"
	}
	s = common.MaskSensitiveInfo(s)
	s = logAnalyticsErrorUUIDPattern.ReplaceAllString(s, "{id}")
	s = logAnalyticsErrorHexPattern.ReplaceAllString(s, "{hex}")
	s = logAnalyticsErrorNumberPattern.ReplaceAllString(s, "{n}")
	s = strings.Join(strings.Fields(s), " ")
	return strings.ToLower(s)
}

func formatLogAnalyticsErrorDisplay(message string) string {
	s := strings.TrimSpace(message)
	if s == "" {
		return "-"
	}
	display := normalizeLogAnalyticsErrorKey(s)
	if len(display) > 240 {
		return display[:240] + "..."
	}
	return display
}

func buildLogAnalyticsErrorClusters(rows []logAnalyticsErrorScanRow) []LogAnalyticsErrorCluster {
	clusterMap := map[logAnalyticsErrorClusterKey]*logAnalyticsErrorClusterAccumulator{}
	for _, row := range rows {
		normalizedKey := normalizeLogAnalyticsErrorKey(row.Message)
		key := logAnalyticsErrorClusterKey{
			normalizedKey: normalizedKey,
			modelName:     row.ModelName,
			channelID:     row.ChannelID,
		}
		acc, ok := clusterMap[key]
		if !ok {
			acc = &logAnalyticsErrorClusterAccumulator{
				message:   formatLogAnalyticsErrorDisplay(row.Message),
				modelName: row.ModelName,
				channelID: row.ChannelID,
			}
			clusterMap[key] = acc
		}
		acc.count++
		if row.CreatedAt > acc.latestAt {
			acc.latestAt = row.CreatedAt
		}
	}

	clusters := make([]LogAnalyticsErrorCluster, 0, len(clusterMap))
	for _, acc := range clusterMap {
		clusters = append(clusters, LogAnalyticsErrorCluster{
			Message:   acc.message,
			Count:     acc.count,
			LatestAt:  acc.latestAt,
			ModelName: acc.modelName,
			ChannelID: acc.channelID,
		})
	}

	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].Count == clusters[j].Count {
			return clusters[i].LatestAt > clusters[j].LatestAt
		}
		return clusters[i].Count > clusters[j].Count
	})
	if len(clusters) > logAnalyticsMaxErrorClusters {
		clusters = clusters[:logAnalyticsMaxErrorClusters]
	}
	return clusters
}
