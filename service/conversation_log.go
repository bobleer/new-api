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

func SaveConversationLog(relayInfo *relaycommon.RelayInfo, usage *dto.Usage) {
	if !model_setting.GetGlobalSettings().ConversationLogEnabled {
		return
	}
	if relayInfo.RelayMode != relayconstant.RelayModeChatCompletions {
		return
	}
	if relayInfo.AssistantReply == "" {
		return
	}

	textReq, ok := relayInfo.Request.(*dto.GeneralOpenAIRequest)
	if !ok || len(textReq.Messages) == 0 {
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
		Messages:         textReq.Messages,
		AssistantReply:   relayInfo.AssistantReply,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}

	gopool.Go(func() {
		writeConversationLog(logDir, entry)
	})
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
