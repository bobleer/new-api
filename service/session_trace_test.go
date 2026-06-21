package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFingerprintMessagePrefixStable(t *testing.T) {
	messagesRaw := `[{"role":"user","content":"hello"},{"role":"assistant","content":"hi"}]`
	fp1 := fingerprintMessagePrefix(messagesRaw, 1)
	fp2 := fingerprintMessagePrefix(messagesRaw, 2)
	assert.NotEmpty(t, fp1)
	assert.NotEmpty(t, fp2)
	assert.NotEqual(t, fp1, fp2)
	assert.Equal(t, fp1, fingerprintMessagePrefix(messagesRaw, 1))
}

func TestExtractMessagesRaw(t *testing.T) {
	openAI := `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`
	assert.Contains(t, extractMessagesRaw(openAI), "hello")

	claude := `{"model":"claude","messages":[{"role":"user","content":"hello"}]}`
	assert.Contains(t, extractMessagesRaw(claude), "hello")

	gemini := `{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`
	assert.Contains(t, extractMessagesRaw(gemini), "hello")
}
