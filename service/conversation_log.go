package service

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/bytedance/gopkg/util/gopool"
)

type ConversationLog struct {
	RequestId        string        `json:"request_id"`
	Timestamp        int64         `json:"timestamp"`
	UserId           int           `json:"user_id"`
	Model            string        `json:"model"`
	Messages         []dto.Message `json:"messages"`
	AssistantReply   string        `json:"assistant_reply"`
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
}

// SaveConversationLog writes one JSON file per request under CONVERSATION_LOG_DIR (or default ./logs/conversations/).
// Callers must only invoke this after a successful relay (HTTP OK / stream completed without handler error, DoResponse succeeded, quota posted)—failed requests must not call it.
// messageSource, when non-nil, is used to build OpenAI-shaped messages; otherwise relayInfo.Request is used.
// For OpenAI chat completions, messageSource should be the effective *dto.GeneralOpenAIRequest (after channel conversion / system prompt).
func SaveConversationLog(relayInfo *relaycommon.RelayInfo, usage *dto.Usage, messageSource ...dto.Request) {
	if !model_setting.GetGlobalSettings().ConversationLogEnabled {
		return
	}
	if relayInfo.AssistantReply == "" {
		return
	}

	var src dto.Request
	if len(messageSource) > 0 && messageSource[0] != nil {
		src = messageSource[0]
	} else {
		src = relayInfo.Request
	}
	if src == nil {
		return
	}

	messages, ok := conversationLogOpenAIMessages(relayInfo, src)
	if !ok || len(messages) == 0 {
		return
	}

	logDir := os.Getenv("CONVERSATION_LOG_DIR")
	if logDir == "" {
		logDir = "./logs/conversations/"
	}

	promptTokens := 0
	completionTokens := 0
	if usage != nil {
		promptTokens = usage.PromptTokens
		completionTokens = usage.CompletionTokens
	}

	entry := ConversationLog{
		RequestId:        relayInfo.RequestId,
		Timestamp:        time.Now().Unix(),
		UserId:           relayInfo.UserId,
		Model:            relayInfo.OriginModelName,
		Messages:         messages,
		AssistantReply:   relayInfo.AssistantReply,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}

	gopool.Go(func() {
		writeConversationLog(logDir, entry)
	})
}

func conversationLogOpenAIMessages(relayInfo *relaycommon.RelayInfo, src dto.Request) ([]dto.Message, bool) {
	switch r := src.(type) {
	case *dto.GeneralOpenAIRequest:
		if relayInfo.RelayMode != relayconstant.RelayModeChatCompletions {
			return nil, false
		}
		if len(r.Messages) == 0 {
			return nil, false
		}
		return r.Messages, true
	case *dto.ClaudeRequest:
		oai, err := ClaudeToOpenAIRequest(*r, relayInfo)
		if err != nil || len(oai.Messages) == 0 {
			return nil, false
		}
		return oai.Messages, true
	case *dto.GeminiChatRequest:
		oai, err := GeminiToOpenAIRequest(r, relayInfo)
		if err != nil || len(oai.Messages) == 0 {
			return nil, false
		}
		return oai.Messages, true
	default:
		return nil, false
	}
}

func writeConversationLog(logDir string, entry ConversationLog) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		common.SysError(fmt.Sprintf("conversation_log: mkdir failed: %v", err))
		return
	}
	filename := filepath.Join(logDir, fmt.Sprintf("%s.json", entry.RequestId))
	data, err := common.Marshal(entry)
	if err != nil {
		common.SysError(fmt.Sprintf("conversation_log: marshal failed: %v", err))
		return
	}
	if err := os.WriteFile(filename, data, 0644); err != nil {
		common.SysError(fmt.Sprintf("conversation_log: write failed: %v", err))
	}
}
