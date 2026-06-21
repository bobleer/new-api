package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLogAnalyticsGroupsByChannel(t *testing.T) {
	now := common.GetTimestamp()
	other := common.MapToJsonStr(map[string]any{"error_detail_id": "ignored"})
	username := "analytics-user-channel-test"

	logs := []Log{
		{UserId: 70001, Username: username, Type: LogTypeConsume, CreatedAt: now, ChannelId: 1, PromptTokens: 10, CompletionTokens: 5, TokenName: "key-a"},
		{UserId: 70001, Username: username, Type: LogTypeConsume, CreatedAt: now, ChannelId: 1, PromptTokens: 20, CompletionTokens: 0, TokenName: "key-a"},
		{UserId: 70001, Username: username, Type: LogTypeError, CreatedAt: now, ChannelId: 1, Content: "err", Other: other, TokenName: "key-a"},
		{UserId: 70001, Username: username, Type: LogTypeConsume, CreatedAt: now, ChannelId: 2, PromptTokens: 100, CompletionTokens: 50, TokenName: "key-b"},
		{UserId: 70002, Username: "analytics-user-other", Type: LogTypeConsume, CreatedAt: now, ChannelId: 2, PromptTokens: 1, CompletionTokens: 1, TokenName: "key-c"},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	result, err := GetLogAnalytics(LogAnalyticsParams{
		UserID:         70001,
		Username:       username,
		StartTimestamp: now - 60,
		EndTimestamp:   now + 60,
		GroupBy:        LogAnalyticsGroupByChannel,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), result.Summary.CallCount)
	assert.Equal(t, int64(185), result.Summary.TokenCount)
	assert.Equal(t, int64(1), result.Summary.FailureCount)
	assert.Equal(t, 25.0, result.Summary.FailureRate)
	require.Len(t, result.Groups, 2)

	assert.Equal(t, 1, result.Groups[0].ChannelID)
	assert.Equal(t, int64(2), result.Groups[0].CallCount)
	assert.Equal(t, int64(1), result.Groups[0].FailureCount)
	assert.Equal(t, 33.33, result.Groups[0].FailureRate)

	assert.Equal(t, 2, result.Groups[1].ChannelID)
	assert.Equal(t, int64(1), result.Groups[1].CallCount)
	assert.Equal(t, int64(0), result.Groups[1].FailureCount)
}

func TestGetLogAnalyticsGroupsByToken(t *testing.T) {
	now := common.GetTimestamp()
	username := "analytics-user-token-test"
	logs := []Log{
		{UserId: 70003, Username: username, Type: LogTypeConsume, CreatedAt: now, TokenId: 11, TokenName: "alpha", PromptTokens: 5, CompletionTokens: 5},
		{UserId: 70003, Username: username, Type: LogTypeError, CreatedAt: now, TokenId: 11, TokenName: "alpha", Content: "failed"},
		{UserId: 70003, Username: username, Type: LogTypeConsume, CreatedAt: now, TokenId: 12, TokenName: "beta", PromptTokens: 7, CompletionTokens: 3},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	result, err := GetLogAnalytics(LogAnalyticsParams{
		UserID:         70003,
		Username:       username,
		StartTimestamp: now - 60,
		EndTimestamp:   now + 60,
		GroupBy:        LogAnalyticsGroupByToken,
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 2)

	assert.Equal(t, "alpha", result.Groups[0].TokenName)
	assert.Equal(t, int64(1), result.Groups[0].CallCount)
	assert.Equal(t, int64(1), result.Groups[0].FailureCount)
	assert.Equal(t, 50.0, result.Groups[0].FailureRate)

	assert.Equal(t, "beta", result.Groups[1].TokenName)
	assert.Equal(t, int64(10), result.Groups[1].TokenCount)
}

func TestGetLogAnalyticsRequiresTimeRange(t *testing.T) {
	_, err := GetLogAnalytics(LogAnalyticsParams{GroupBy: LogAnalyticsGroupByChannel})
	require.Error(t, err)
}

func TestGetLogAnalyticsRejectsLongRange(t *testing.T) {
	now := common.GetTimestamp()
	_, err := GetLogAnalytics(LogAnalyticsParams{
		StartTimestamp: now - maxLogAnalyticsRangeSeconds - 3600,
		EndTimestamp:   now,
		GroupBy:        LogAnalyticsGroupByChannel,
	})
	require.Error(t, err)
}

func TestNormalizeLogAnalyticsErrorKeyClustersSimilarMessages(t *testing.T) {
	a := normalizeLogAnalyticsErrorKey("Rate limit exceeded for request 9f3a2b1c-4d5e-6f70-8192-abcdef012345")
	b := normalizeLogAnalyticsErrorKey("rate limit exceeded for request 11111111-2222-3333-4444-555555555555")
	assert.Equal(t, a, b)
	assert.Contains(t, a, "rate limit exceeded")
	assert.Contains(t, a, "{id}")
}

func TestBuildLogAnalyticsErrorClusters(t *testing.T) {
	rows := []logAnalyticsErrorScanRow{
		{Message: "upstream timeout after 120 seconds", ModelName: "gpt-4", ChannelID: 1, CreatedAt: 100},
		{Message: "upstream timeout after 180 seconds", ModelName: "gpt-4", ChannelID: 1, CreatedAt: 200},
		{Message: "invalid api key", ModelName: "claude-3", ChannelID: 2, CreatedAt: 300},
	}
	clusters := buildLogAnalyticsErrorClusters(rows)
	require.Len(t, clusters, 2)
	assert.Equal(t, int64(2), clusters[0].Count)
	assert.Contains(t, clusters[0].Message, "upstream timeout")
	assert.Equal(t, int64(1), clusters[1].Count)
}

func TestGetLogAnalyticsInsights(t *testing.T) {
	now := common.GetTimestamp()
	username := "analytics-user-insights-test"
	base := now - 3600
	logs := []Log{
		{UserId: 70004, Username: username, Type: LogTypeConsume, CreatedAt: base + 600, ChannelId: 3, ModelName: "gpt-4", Group: "default", PromptTokens: 10, CompletionTokens: 5},
		{UserId: 70004, Username: username, Type: LogTypeError, CreatedAt: base + 1200, ChannelId: 3, ModelName: "gpt-4", Group: "default", Content: "rate limit exceeded"},
		{UserId: 70004, Username: username, Type: LogTypeError, CreatedAt: base + 1800, ChannelId: 3, ModelName: "gpt-4", Group: "default", Content: "rate limit exceeded"},
		{UserId: 70004, Username: username, Type: LogTypeConsume, CreatedAt: base + 2400, ChannelId: 4, ModelName: "claude-3", Group: "vip", PromptTokens: 20, CompletionTokens: 10},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	result, err := GetLogAnalytics(LogAnalyticsParams{
		UserID:         70004,
		Username:       username,
		StartTimestamp: base,
		EndTimestamp:   now,
		GroupBy:        LogAnalyticsGroupByChannel,
	})
	require.NoError(t, err)
	require.NotNil(t, result.Insights)
	assert.NotEmpty(t, result.Insights.TimeSeries)
	assert.NotEmpty(t, result.Insights.Heatmap)
	require.NotEmpty(t, result.Insights.Errors)
	assert.Equal(t, int64(2), result.Insights.Errors[0].Count)
	assert.Contains(t, result.Insights.Errors[0].Message, "rate limit")
	assert.NotEmpty(t, result.Insights.FlowLinks)
}
