package model

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	logAnalyticsMaxErrorClusters = 20
	logAnalyticsMaxFlowLinks     = 80
)

type LogAnalyticsTimeSeriesPoint struct {
	BucketStart  int64   `json:"bucket_start"`
	CallCount    int64   `json:"call_count"`
	FailureCount int64   `json:"failure_count"`
	TokenCount   int64   `json:"token_count"`
	FailureRate  float64 `json:"failure_rate"`
}

type LogAnalyticsHeatmapCell struct {
	Weekday      int     `json:"weekday"`
	Hour         int     `json:"hour"`
	CallCount    int64   `json:"call_count"`
	FailureCount int64   `json:"failure_count"`
	FailureRate  float64 `json:"failure_rate"`
}

type LogAnalyticsErrorCluster struct {
	Message     string `json:"message"`
	Count       int64  `json:"count"`
	LatestAt    int64  `json:"latest_at"`
	ModelName   string `json:"model_name,omitempty"`
	ChannelID   int    `json:"channel_id,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
}

type LogAnalyticsFlowLink struct {
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceKind   string `json:"source_kind"`
	TargetKind   string `json:"target_kind"`
	CallCount    int64  `json:"call_count"`
	FailureCount int64  `json:"failure_count"`
}

type LogAnalyticsInsights struct {
	BucketSeconds int64                         `json:"bucket_seconds"`
	TimeSeries    []LogAnalyticsTimeSeriesPoint `json:"time_series"`
	Heatmap       []LogAnalyticsHeatmapCell     `json:"heatmap"`
	Errors        []LogAnalyticsErrorCluster    `json:"errors"`
	FlowLinks     []LogAnalyticsFlowLink        `json:"flow_links"`
}

type logAnalyticsBucketRow struct {
	BucketStart  int64 `gorm:"column:bucket_start"`
	CallCount    int64 `gorm:"column:call_count"`
	FailureCount int64 `gorm:"column:failure_count"`
	TokenCount   int64 `gorm:"column:token_count"`
}

type logAnalyticsErrorRow struct {
	Message   string `gorm:"column:message"`
	ModelName string `gorm:"column:model_name"`
	ChannelID int    `gorm:"column:channel_id"`
	Count     int64  `gorm:"column:count"`
	LatestAt  int64  `gorm:"column:latest_at"`
}

type logAnalyticsFlowRow struct {
	Group        string `gorm:"column:group_name"`
	ModelName    string `gorm:"column:model_name"`
	ChannelID    int    `gorm:"column:channel_id"`
	CallCount    int64  `gorm:"column:call_count"`
	FailureCount int64  `gorm:"column:failure_count"`
}

type logAnalyticsFlowEdgeKey struct {
	source     string
	target     string
	sourceKind string
	targetKind string
}

func selectAnalyticsBucketSeconds(startTimestamp, endTimestamp int64) int64 {
	span := endTimestamp - startTimestamp
	switch {
	case span <= 2*86400:
		return 3600
	case span <= 14*86400:
		return 6 * 3600
	default:
		return 86400
	}
}

func buildLogAnalyticsInsights(filteredTx *gorm.DB, params LogAnalyticsParams) (*LogAnalyticsInsights, error) {
	bucketSeconds := selectAnalyticsBucketSeconds(params.StartTimestamp, params.EndTimestamp)
	aggregateSelect := `
SUM(CASE WHEN logs.type = ? THEN 1 ELSE 0 END) AS call_count,
SUM(CASE WHEN logs.type = ? THEN 1 ELSE 0 END) AS failure_count,
SUM(CASE WHEN logs.type = ? THEN COALESCE(logs.prompt_tokens, 0) + COALESCE(logs.completion_tokens, 0) ELSE 0 END) AS token_count`

	bucketExpr := "(logs.created_at / " + strconv.FormatInt(bucketSeconds, 10) + ") * " + strconv.FormatInt(bucketSeconds, 10)
	var bucketRows []logAnalyticsBucketRow
	bucketTx := filteredTx.Session(&gorm.Session{}).
		Select(bucketExpr+" AS bucket_start, "+aggregateSelect, LogTypeConsume, LogTypeError, LogTypeConsume).
		Group("bucket_start").
		Order("bucket_start ASC")
	if err := bucketTx.Scan(&bucketRows).Error; err != nil {
		return nil, err
	}

	timeSeries := make([]LogAnalyticsTimeSeriesPoint, 0, len(bucketRows))
	heatmapMap := map[int64]*LogAnalyticsHeatmapCell{}
	for _, row := range bucketRows {
		timeSeries = append(timeSeries, LogAnalyticsTimeSeriesPoint{
			BucketStart:  row.BucketStart,
			CallCount:    row.CallCount,
			FailureCount: row.FailureCount,
			TokenCount:   row.TokenCount,
			FailureRate:  computeFailureRate(row.CallCount, row.FailureCount),
		})
		tm := time.Unix(row.BucketStart, 0).UTC()
		heatmapKey := int64(tm.Weekday())*100 + int64(tm.Hour())
		cell, ok := heatmapMap[heatmapKey]
		if !ok {
			cell = &LogAnalyticsHeatmapCell{
				Weekday: int(tm.Weekday()),
				Hour:    tm.Hour(),
			}
			heatmapMap[heatmapKey] = cell
		}
		cell.CallCount += row.CallCount
		cell.FailureCount += row.FailureCount
	}

	heatmap := make([]LogAnalyticsHeatmapCell, 0, len(heatmapMap))
	for _, cell := range heatmapMap {
		cell.FailureRate = computeFailureRate(cell.CallCount, cell.FailureCount)
		heatmap = append(heatmap, *cell)
	}
	sort.Slice(heatmap, func(i, j int) bool {
		if heatmap[i].Weekday == heatmap[j].Weekday {
			return heatmap[i].Hour < heatmap[j].Hour
		}
		return heatmap[i].Weekday < heatmap[j].Weekday
	})

	var errorRows []logAnalyticsErrorRow
	errorTx := filteredTx.Session(&gorm.Session{}).
		Where("logs.type = ?", LogTypeError).
		Select(`
logs.content AS message,
logs.model_name AS model_name,
logs.channel_id AS channel_id,
COUNT(*) AS count,
MAX(logs.created_at) AS latest_at`).
		Group("logs.content, logs.model_name, logs.channel_id").
		Order("count DESC, latest_at DESC").
		Limit(logAnalyticsMaxErrorClusters)
	if err := errorTx.Scan(&errorRows).Error; err != nil {
		return nil, err
	}

	channelIDs := make([]int, 0, len(errorRows))
	for _, row := range errorRows {
		if row.ChannelID != 0 {
			channelIDs = append(channelIDs, row.ChannelID)
		}
	}
	channelNames := loadChannelNameMap(channelIDs)

	errors := make([]LogAnalyticsErrorCluster, 0, len(errorRows))
	for _, row := range errorRows {
		message := strings.TrimSpace(row.Message)
		if message == "" {
			message = "-"
		}
		if len(message) > 240 {
			message = message[:240] + "..."
		}
		errors = append(errors, LogAnalyticsErrorCluster{
			Message:     message,
			Count:       row.Count,
			LatestAt:    row.LatestAt,
			ModelName:   row.ModelName,
			ChannelID:   row.ChannelID,
			ChannelName: channelNames[row.ChannelID],
		})
	}

	groupSelect := "COALESCE(NULLIF(logs." + logGroupCol + ", ''), '-') AS group_name, logs.model_name AS model_name, logs.channel_id AS channel_id, " + aggregateSelect
	var flowRows []logAnalyticsFlowRow
	flowTx := filteredTx.Session(&gorm.Session{}).
		Select(groupSelect, LogTypeConsume, LogTypeError, LogTypeConsume).
		Group("group_name, logs.model_name, logs.channel_id")
	if err := flowTx.Scan(&flowRows).Error; err != nil {
		return nil, err
	}

	type flowEdgeKey = logAnalyticsFlowEdgeKey
	flowEdgeMap := map[flowEdgeKey]*LogAnalyticsFlowLink{}
	for _, row := range flowRows {
		groupLabel := strings.TrimSpace(row.Group)
		if groupLabel == "" {
			groupLabel = "-"
		}
		modelLabel := strings.TrimSpace(row.ModelName)
		if modelLabel == "" {
			modelLabel = "-"
		}
		channelLabel := formatAnalyticsChannelLabel(row.ChannelID)

		addFlowEdge(flowEdgeMap, groupLabel, modelLabel, "group", "model", row.CallCount, row.FailureCount)
		addFlowEdge(flowEdgeMap, modelLabel, channelLabel, "model", "channel", row.CallCount, row.FailureCount)
	}

	flowLinks := make([]LogAnalyticsFlowLink, 0, len(flowEdgeMap))
	for _, link := range flowEdgeMap {
		flowLinks = append(flowLinks, *link)
	}
	sort.Slice(flowLinks, func(i, j int) bool {
		if flowLinks[i].CallCount+flowLinks[i].FailureCount == flowLinks[j].CallCount+flowLinks[j].FailureCount {
			return flowLinks[i].FailureCount > flowLinks[j].FailureCount
		}
		return flowLinks[i].CallCount+flowLinks[i].FailureCount > flowLinks[j].CallCount+flowLinks[j].FailureCount
	})
	if len(flowLinks) > logAnalyticsMaxFlowLinks {
		flowLinks = flowLinks[:logAnalyticsMaxFlowLinks]
	}

	return &LogAnalyticsInsights{
		BucketSeconds: bucketSeconds,
		TimeSeries:    timeSeries,
		Heatmap:       heatmap,
		Errors:        errors,
		FlowLinks:     flowLinks,
	}, nil
}

func addFlowEdge(
	flowEdgeMap map[logAnalyticsFlowEdgeKey]*LogAnalyticsFlowLink,
	source, target, sourceKind, targetKind string,
	callCount, failureCount int64,
) {
	key := logAnalyticsFlowEdgeKey{
		source:     source,
		target:     target,
		sourceKind: sourceKind,
		targetKind: targetKind,
	}
	link, ok := flowEdgeMap[key]
	if !ok {
		link = &LogAnalyticsFlowLink{
			Source:     source,
			Target:     target,
			SourceKind: sourceKind,
			TargetKind: targetKind,
		}
		flowEdgeMap[key] = link
	}
	link.CallCount += callCount
	link.FailureCount += failureCount
}

func formatAnalyticsChannelLabel(channelID int) string {
	if channelID == 0 {
		return "-"
	}
	return "CH-" + strconv.Itoa(channelID)
}
