package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionTraceSaveLoadAndView(t *testing.T) {
	tempDir := t.TempDir()
	constant.SessionTraceDir = tempDir
	constant.SessionTraceMaxMB = 1
	require.NoError(t, LOG_DB.AutoMigrate(&SessionTrace{}, &SessionTraceTurn{}))

	traceID := NewTraceID()
	now := time.Now().Unix()

	session := &SessionTrace{
		TraceId:        traceID,
		UserId:         1,
		TokenId:        2,
		ModelName:      "gpt-4",
		TurnCount:      1,
		CreatedAt:      now,
		LastActivityAt: now,
	}
	require.NoError(t, UpsertSessionTrace(session))

	turn := &SessionTraceTurn{
		TraceId:          traceID,
		TurnIndex:        1,
		RequestId:        "req-1",
		UserId:           1,
		TokenId:          2,
		ModelName:        "gpt-4",
		ChannelId:        3,
		Status:           "success",
		PromptTokens:     10,
		CompletionTokens: 5,
		CreatedAt:        now,
	}
	require.NoError(t, InsertSessionTraceTurn(turn))

	detail := BuildSessionTraceTurnDetailPayload(
		traceID,
		1,
		"req-1",
		`{"messages":[{"role":"user","content":"hello"}]}`,
		`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`,
		false,
	)
	require.NoError(t, SaveSessionTraceTurnDetail(detail))

	loaded, err := LoadSessionTraceTurnDetail(traceID, 1)
	require.NoError(t, err)
	assert.Contains(t, loaded.ClientRequest, "hello")
	assert.Contains(t, loaded.AssistantResponse, "hi")

	view, err := BuildSessionTraceView(traceID, false)
	require.NoError(t, err)
	assert.Equal(t, traceID, view.TraceId)
	require.Len(t, view.Turns, 1)
	assert.True(t, view.Turns[0].HasDetail)

	_, err = os.Stat(filepath.Join(tempDir, traceID, "1.json"))
	require.NoError(t, err)
}

func TestIsValidTraceID(t *testing.T) {
	assert.True(t, IsValidTraceID(NewTraceID()))
	assert.False(t, IsValidTraceID("not-a-uuid"))
}

func TestBuildSessionTraceTurnDetailPayloadTruncatesLargeFields(t *testing.T) {
	constant.SessionTraceMaxMB = 1
	large := common.GetRandomString(2 << 20)
	detail := BuildSessionTraceTurnDetailPayload(
		NewTraceID(),
		1,
		"req-1",
		large,
		large,
		true,
	)
	assert.True(t, detail.Truncated)
	assert.LessOrEqual(t, len(detail.ClientRequest), (1<<20))
	assert.LessOrEqual(t, len(detail.AssistantResponse), (1<<20))
}
