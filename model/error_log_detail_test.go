package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorLogDetailSaveLoadAndLookup(t *testing.T) {
	tempDir := t.TempDir()
	constant.ErrorLogDetailDir = tempDir
	constant.ErrorLogDetailMaxMB = 1

	detailID := NewErrorDetailID()
	detail := BuildErrorLogDetailPayload(
		detailID,
		42,
		"status_code=502, upstream unavailable",
		"upstream_error",
		"bad_response_status_code",
		502,
		`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`,
		`{"error":{"message":"upstream unavailable"}}`,
		"/v1/chat/completions",
		"gpt-4",
		7,
		"test-channel",
		true,
		"req-123",
		"upstream-req-456",
	)
	require.NoError(t, SaveErrorLogDetail(detail))

	loaded, err := LoadErrorLogDetail(detailID)
	require.NoError(t, err)
	assert.Equal(t, detailID, loaded.DetailID)
	assert.Equal(t, 42, loaded.UserID)
	assert.Contains(t, loaded.ClientRequest, `"messages"`)
	assert.Contains(t, loaded.UpstreamResponse, "upstream unavailable")
	assert.Equal(t, "req-123", loaded.RequestID)

	other := common.MapToJsonStr(map[string]any{
		"error_detail_id": detailID,
	})
	log := &Log{
		UserId:    42,
		Type:      LogTypeError,
		Content:   "status_code=502",
		CreatedAt: common.GetTimestamp(),
		Other:     other,
	}
	require.NoError(t, LOG_DB.Create(log).Error)

	found, err := FindLogByErrorDetailID(detailID)
	require.NoError(t, err)
	assert.Equal(t, log.Id, found.Id)
	assert.Equal(t, 42, found.UserId)

	require.NoError(t, DeleteErrorLogDetail(detailID))
	_, err = os.Stat(filepath.Join(tempDir, detailID+".json"))
	assert.True(t, os.IsNotExist(err))
}

func TestBuildErrorLogDetailPayloadTruncatesLargeFields(t *testing.T) {
	constant.ErrorLogDetailMaxMB = 1
	large := make([]byte, (1<<20)+128)
	for i := range large {
		large[i] = 'a'
	}

	detail := BuildErrorLogDetailPayload(
		NewErrorDetailID(),
		1,
		string(large),
		"upstream_error",
		"bad_response_status_code",
		500,
		string(large),
		string(large),
		"/v1/chat/completions",
		"gpt-4",
		1,
		"channel",
		false,
		"",
		"",
	)

	assert.True(t, detail.Truncated)
	assert.LessOrEqual(t, len(detail.ClientRequest), (1<<20))
	assert.LessOrEqual(t, len(detail.UpstreamResponse), (1<<20))
}

func TestIsValidErrorDetailID(t *testing.T) {
	assert.True(t, IsValidErrorDetailID(NewErrorDetailID()))
	assert.False(t, IsValidErrorDetailID("../secret"))
	assert.False(t, IsValidErrorDetailID(""))
}
