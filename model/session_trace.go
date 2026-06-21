package model

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var traceIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type SessionTrace struct {
	TraceId        string `json:"trace_id" gorm:"primaryKey;type:varchar(36)"`
	UserId         int    `json:"user_id" gorm:"index"`
	TokenId        int    `json:"token_id" gorm:"index"`
	ModelName      string `json:"model_name" gorm:"type:varchar(128);default:''"`
	TurnCount      int    `json:"turn_count" gorm:"default:0"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index"`
	LastActivityAt int64  `json:"last_activity_at" gorm:"bigint;index"`
}

type SessionTraceTurn struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	TraceId          string `json:"trace_id" gorm:"type:varchar(36);index:idx_session_trace_turns_trace_turn,priority:1"`
	TurnIndex        int    `json:"turn_index" gorm:"index:idx_session_trace_turns_trace_turn,priority:2"`
	RequestId        string `json:"request_id" gorm:"type:varchar(64);index;default:''"`
	UserId           int    `json:"user_id" gorm:"index"`
	TokenId          int    `json:"token_id" gorm:"default:0;index"`
	ModelName        string `json:"model_name" gorm:"type:varchar(128);default:''"`
	ChannelId        int    `json:"channel_id" gorm:"default:0"`
	Status           string `json:"status" gorm:"type:varchar(16);default:''"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int    `json:"completion_tokens" gorm:"default:0"`
	IsStream         bool   `json:"is_stream"`
	ErrorMessage     string `json:"error_message,omitempty" gorm:"type:text;default:''"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index"`
}

type SessionTraceTurnDetail struct {
	TraceId           string `json:"trace_id"`
	TurnIndex         int    `json:"turn_index"`
	RequestId         string `json:"request_id,omitempty"`
	ClientRequest     string `json:"client_request,omitempty"`
	AssistantResponse string `json:"assistant_response,omitempty"`
	IsStream          bool   `json:"is_stream,omitempty"`
	Truncated         bool   `json:"truncated,omitempty"`
}

type SessionTraceView struct {
	SessionTrace
	Turns []SessionTraceTurnView `json:"turns"`
}

type SessionTraceTurnView struct {
	SessionTraceTurn
	HasDetail bool `json:"has_detail"`
}

func NewTraceID() string {
	return uuid.NewString()
}

func IsValidTraceID(id string) bool {
	return traceIDPattern.MatchString(id)
}

func sessionTraceMaxBytes() int {
	maxMB := constant.SessionTraceMaxMB
	if maxMB <= 0 {
		maxMB = 4
	}
	return maxMB << 20
}

func sessionTraceDir() string {
	if constant.SessionTraceDir != "" {
		return constant.SessionTraceDir
	}
	return "./data/session-traces"
}

func ensureSessionTraceDir(traceID string) error {
	dir := filepath.Join(sessionTraceDir(), traceID)
	if _, err := os.Stat(dir); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(dir, 0755)
}

func sessionTraceTurnDetailPath(traceID string, turnIndex int) string {
	return filepath.Join(sessionTraceDir(), traceID, fmt.Sprintf("%d.json", turnIndex))
}

func truncateForSessionTrace(value string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value, false
	}
	return value[:maxBytes], true
}

func maskAndLimitSessionTraceText(value string, maxBytes int) (string, bool) {
	masked := common.MaskSensitiveInfo(value)
	return truncateForSessionTrace(masked, maxBytes)
}

func SaveSessionTraceTurnDetail(detail *SessionTraceTurnDetail) error {
	if detail == nil {
		return errors.New("session trace turn detail is nil")
	}
	if !IsValidTraceID(detail.TraceId) {
		return errors.New("invalid trace id")
	}
	if detail.TurnIndex <= 0 {
		return errors.New("invalid turn index")
	}
	if err := ensureSessionTraceDir(detail.TraceId); err != nil {
		return err
	}
	data, err := common.Marshal(detail)
	if err != nil {
		return err
	}
	return os.WriteFile(sessionTraceTurnDetailPath(detail.TraceId, detail.TurnIndex), data, 0644)
}

func LoadSessionTraceTurnDetail(traceID string, turnIndex int) (*SessionTraceTurnDetail, error) {
	if !IsValidTraceID(traceID) {
		return nil, errors.New("invalid trace id")
	}
	if turnIndex <= 0 {
		return nil, errors.New("invalid turn index")
	}
	data, err := os.ReadFile(sessionTraceTurnDetailPath(traceID, turnIndex))
	if err != nil {
		return nil, err
	}
	var detail SessionTraceTurnDetail
	if err := common.Unmarshal(data, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func sessionTraceTurnDetailExists(traceID string, turnIndex int) bool {
	_, err := os.Stat(sessionTraceTurnDetailPath(traceID, turnIndex))
	return err == nil
}

func UpsertSessionTrace(session *SessionTrace) error {
	if session == nil || !IsValidTraceID(session.TraceId) {
		return errors.New("invalid session trace")
	}
	var existing SessionTrace
	err := LOG_DB.Where("trace_id = ?", session.TraceId).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return LOG_DB.Create(session).Error
	}
	if err != nil {
		return err
	}
	return LOG_DB.Model(&existing).Updates(map[string]interface{}{
		"turn_count":       session.TurnCount,
		"last_activity_at": session.LastActivityAt,
		"model_name":       session.ModelName,
	}).Error
}

func GetSessionTrace(traceID string) (*SessionTrace, error) {
	if !IsValidTraceID(traceID) {
		return nil, errors.New("invalid trace id")
	}
	var session SessionTrace
	if err := LOG_DB.Where("trace_id = ?", traceID).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func ListSessionTraceTurns(traceID string) ([]SessionTraceTurn, error) {
	if !IsValidTraceID(traceID) {
		return nil, errors.New("invalid trace id")
	}
	var turns []SessionTraceTurn
	err := LOG_DB.Where("trace_id = ?", traceID).Order("turn_index asc").Find(&turns).Error
	return turns, err
}

func InsertSessionTraceTurn(turn *SessionTraceTurn) error {
	if turn == nil || !IsValidTraceID(turn.TraceId) {
		return errors.New("invalid session trace turn")
	}
	return LOG_DB.Create(turn).Error
}

func BuildSessionTraceView(traceID string, includeDetails bool) (*SessionTraceView, error) {
	session, err := GetSessionTrace(traceID)
	if err != nil {
		return nil, err
	}
	turns, err := ListSessionTraceTurns(traceID)
	if err != nil {
		return nil, err
	}
	view := &SessionTraceView{
		SessionTrace: *session,
		Turns:        make([]SessionTraceTurnView, 0, len(turns)),
	}
	for _, turn := range turns {
		turnView := SessionTraceTurnView{
			SessionTraceTurn: turn,
			HasDetail:        sessionTraceTurnDetailExists(traceID, turn.TurnIndex),
		}
		view.Turns = append(view.Turns, turnView)
	}
	if includeDetails {
		for i := range view.Turns {
			detail, loadErr := LoadSessionTraceTurnDetail(traceID, view.Turns[i].TurnIndex)
			if loadErr != nil {
				continue
			}
			view.Turns[i].ErrorMessage = strings.TrimSpace(view.Turns[i].ErrorMessage)
			_ = detail
		}
	}
	return view, nil
}

func BuildSessionTraceTurnDetailPayload(
	traceID string,
	turnIndex int,
	requestID string,
	clientRequest string,
	assistantResponse string,
	isStream bool,
) *SessionTraceTurnDetail {
	maxBytes := sessionTraceMaxBytes() / 2
	clientReq, clientTrunc := maskAndLimitSessionTraceText(clientRequest, maxBytes)
	assistantResp, assistantTrunc := maskAndLimitSessionTraceText(assistantResponse, maxBytes)
	return &SessionTraceTurnDetail{
		TraceId:           traceID,
		TurnIndex:         turnIndex,
		RequestId:         requestID,
		ClientRequest:     clientReq,
		AssistantResponse: assistantResp,
		IsStream:          isStream,
		Truncated:         clientTrunc || assistantTrunc,
	}
}
