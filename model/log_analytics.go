package model

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"gorm.io/gorm"
)

const (
	LogAnalyticsGroupByChannel = "channel"
	LogAnalyticsGroupByToken   = "token"
)

type LogAnalyticsParams struct {
	UserID         int
	StartTimestamp int64
	EndTimestamp   int64
	GroupBy        string
	Username       string
	TokenName      string
	ModelName      string
	Channel        int
	Group          string
}

type LogAnalyticsSummary struct {
	CallCount    int64   `json:"call_count"`
	TokenCount   int64   `json:"token_count"`
	FailureCount int64   `json:"failure_count"`
	FailureRate  float64 `json:"failure_rate"`
}

type LogAnalyticsGroupRow struct {
	ChannelID    int     `json:"channel_id,omitempty"`
	ChannelName  string  `json:"channel_name,omitempty"`
	TokenID      int     `json:"token_id,omitempty"`
	TokenName    string  `json:"token_name,omitempty"`
	CallCount    int64   `json:"call_count"`
	TokenCount   int64   `json:"token_count"`
	FailureCount int64   `json:"failure_count"`
	FailureRate  float64 `json:"failure_rate"`
}

type LogAnalyticsResult struct {
	Summary  LogAnalyticsSummary   `json:"summary"`
	Groups   []LogAnalyticsGroupRow `json:"groups"`
	Insights *LogAnalyticsInsights  `json:"insights,omitempty"`
}

type logAnalyticsAggregateRow struct {
	ChannelID    int    `gorm:"column:channel_id"`
	TokenID      int    `gorm:"column:token_id"`
	TokenName    string `gorm:"column:token_name"`
	CallCount    int64  `gorm:"column:call_count"`
	TokenCount   int64  `gorm:"column:token_count"`
	FailureCount int64  `gorm:"column:failure_count"`
}

func computeFailureRate(callCount, failureCount int64) float64 {
	total := callCount + failureCount
	if total <= 0 {
		return 0
	}
	return math.Round((float64(failureCount)/float64(total))*10000) / 100
}

func applyLogAnalyticsFilters(tx *gorm.DB, params LogAnalyticsParams) (*gorm.DB, error) {
	if params.UserID > 0 {
		tx = tx.Where("logs.user_id = ?", params.UserID)
	}
	if params.StartTimestamp > 0 {
		tx = tx.Where("logs.created_at >= ?", params.StartTimestamp)
	}
	if params.EndTimestamp > 0 {
		tx = tx.Where("logs.created_at <= ?", params.EndTimestamp)
	}
	if params.Channel > 0 {
		tx = tx.Where("logs.channel_id = ?", params.Channel)
	}
	if params.Group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", params.Group)
	}

	var err error
	if tx, err = applyExplicitLogTextFilter(tx, "logs.username", params.Username); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", params.ModelName); err != nil {
		return nil, err
	}
	if params.TokenName != "" {
		tx = tx.Where("logs.token_name = ?", params.TokenName)
	}
	tx = tx.Where("logs.type IN ?", []int{LogTypeConsume, LogTypeError})
	return tx, nil
}

func GetLogAnalytics(params LogAnalyticsParams) (*LogAnalyticsResult, error) {
	if params.StartTimestamp <= 0 || params.EndTimestamp <= 0 {
		return nil, errors.New("start_timestamp and end_timestamp are required")
	}
	if params.StartTimestamp > params.EndTimestamp {
		return nil, errors.New("start_timestamp must be before end_timestamp")
	}
	switch params.GroupBy {
	case LogAnalyticsGroupByChannel, LogAnalyticsGroupByToken:
	default:
		return nil, fmt.Errorf("unsupported group_by: %s", params.GroupBy)
	}

	aggregateSelect := `
SUM(CASE WHEN logs.type = ? THEN 1 ELSE 0 END) AS call_count,
SUM(CASE WHEN logs.type = ? THEN COALESCE(logs.prompt_tokens, 0) + COALESCE(logs.completion_tokens, 0) ELSE 0 END) AS token_count,
SUM(CASE WHEN logs.type = ? THEN 1 ELSE 0 END) AS failure_count`

	baseTx := LOG_DB.Table("logs")
	filteredTx, err := applyLogAnalyticsFilters(baseTx, params)
	if err != nil {
		return nil, err
	}

	summaryTx := filteredTx.Session(&gorm.Session{}).
		Select(aggregateSelect, LogTypeConsume, LogTypeConsume, LogTypeError)
	var summaryRow logAnalyticsAggregateRow
	if err := summaryTx.Scan(&summaryRow).Error; err != nil {
		return nil, err
	}

	result := &LogAnalyticsResult{
		Summary: LogAnalyticsSummary{
			CallCount:    summaryRow.CallCount,
			TokenCount:   summaryRow.TokenCount,
			FailureCount: summaryRow.FailureCount,
			FailureRate:  computeFailureRate(summaryRow.CallCount, summaryRow.FailureCount),
		},
		Groups: []LogAnalyticsGroupRow{},
	}

	groupSelect := aggregateSelect
	groupBy := ""
	if params.GroupBy == LogAnalyticsGroupByChannel {
		groupSelect = "logs.channel_id AS channel_id, " + groupSelect
		groupBy = "logs.channel_id"
	} else {
		groupSelect = "logs.token_id AS token_id, logs.token_name AS token_name, " + groupSelect
		groupBy = "logs.token_id, logs.token_name"
	}

	var rows []logAnalyticsAggregateRow
	groupTx := filteredTx.Session(&gorm.Session{}).
		Select(groupSelect, LogTypeConsume, LogTypeConsume, LogTypeError).
		Group(groupBy).
		Order("call_count DESC, failure_count DESC")
	if err := groupTx.Scan(&rows).Error; err != nil {
		return nil, err
	}

	channelNames := map[int]string{}
	if params.GroupBy == LogAnalyticsGroupByChannel {
		channelIDs := types.NewSet[int]()
		for _, row := range rows {
			if row.ChannelID != 0 {
				channelIDs.Add(row.ChannelID)
			}
		}
		channelNames = loadChannelNameMap(channelIDs.Items())
	}

	for _, row := range rows {
		groupRow := LogAnalyticsGroupRow{
			ChannelID:    row.ChannelID,
			ChannelName:  channelNames[row.ChannelID],
			TokenID:      row.TokenID,
			TokenName:    row.TokenName,
			CallCount:    row.CallCount,
			TokenCount:   row.TokenCount,
			FailureCount: row.FailureCount,
			FailureRate:  computeFailureRate(row.CallCount, row.FailureCount),
		}
		if params.GroupBy == LogAnalyticsGroupByToken && groupRow.TokenName == "" && groupRow.TokenID == 0 {
			groupRow.TokenName = "-"
		}
		result.Groups = append(result.Groups, groupRow)
	}

	insights, err := buildLogAnalyticsInsights(filteredTx, params)
	if err != nil {
		return nil, err
	}
	result.Insights = insights

	return result, nil
}

func loadChannelNameMap(channelIDs []int) map[int]string {
	channelMap := make(map[int]string, len(channelIDs))
	if len(channelIDs) == 0 {
		return channelMap
	}

	if common.MemoryCacheEnabled {
		for _, channelID := range channelIDs {
			if cacheChannel, err := CacheGetChannel(channelID); err == nil {
				channelMap[channelID] = cacheChannel.Name
			}
		}
		return channelMap
	}

	var channels []struct {
		Id   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if err := DB.Table("channels").Select("id, name").Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		common.SysLog("failed to load channel names for log analytics: " + err.Error())
		return channelMap
	}
	for _, channel := range channels {
		channelMap[channel.Id] = channel.Name
	}
	return channelMap
}
