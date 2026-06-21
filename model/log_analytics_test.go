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
