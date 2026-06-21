package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/samber/hot"
	"github.com/tidwall/gjson"
)

const (
	sessionTraceCacheNamespace = "new-api:session_trace:v1"
	ginKeySessionTraceWriter   = "session_trace_response_writer"
)

var (
	sessionTraceCacheOnce sync.Once
	sessionTraceCache     *cachex.HybridCache[string]
)

type SessionTraceTurnMeta struct {
	PromptTokens     int
	CompletionTokens int
}

type SessionTraceTurnDetailView struct {
	model.SessionTraceTurn
	Detail *model.SessionTraceTurnDetail `json:"detail,omitempty"`
}

type SessionTraceFullView struct {
	model.SessionTrace
	Turns []SessionTraceTurnDetailView `json:"turns"`
}

func sessionTraceCacheInstance() *cachex.HybridCache[string] {
	sessionTraceCacheOnce.Do(func() {
		sessionTraceCache = cachex.NewHybridCache[string](cachex.HybridCacheConfig[string]{
			Namespace:  sessionTraceCacheNamespace,
			Redis:      common.RDB,
			RedisCodec: cachex.StringCodec{},
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			Memory: func() *hot.HotCache[string, string] {
				return hot.NewHotCache[string, string](hot.LRU, 10000).Build()
			},
		})
	})
	return sessionTraceCache
}

func sessionTraceTTL() time.Duration {
	days := constant.SessionTraceTTLDays
	if days <= 0 {
		days = 7
	}
	return time.Duration(days) * 24 * time.Hour
}

func extractMessagesRaw(body string) string {
	if body == "" {
		return ""
	}
	if messages := gjson.Get(body, "messages"); messages.Exists() && messages.IsArray() {
		return messages.Raw
	}
	if contents := gjson.Get(body, "contents"); contents.Exists() && contents.IsArray() {
		return contents.Raw
	}
	if input := gjson.Get(body, "input"); input.Exists() && input.IsArray() {
		return input.Raw
	}
	return ""
}

func fingerprintMessagePrefix(messagesRaw string, prefixLen int) string {
	if prefixLen <= 0 || messagesRaw == "" {
		return ""
	}
	arr := gjson.Parse(messagesRaw)
	if !arr.IsArray() {
		return ""
	}
	items := arr.Array()
	if prefixLen > len(items) {
		prefixLen = len(items)
	}
	parts := make([]string, 0, prefixLen)
	for i := 0; i < prefixLen; i++ {
		parts = append(parts, items[i].Raw)
	}
	combined := "[" + strings.Join(parts, ",") + "]"
	sum := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(sum[:])
}

func sessionTraceLookupKey(tokenID int, modelName, fingerprint string) string {
	return fmt.Sprintf("%d:%s:%s", tokenID, modelName, fingerprint)
}

func lookupTraceID(tokenID int, modelName, messagesRaw string) (string, bool) {
	if messagesRaw == "" {
		return "", false
	}
	arr := gjson.Parse(messagesRaw)
	if !arr.IsArray() {
		return "", false
	}
	n := len(arr.Array())
	cache := sessionTraceCacheInstance()
	for i := n - 1; i >= 1; i-- {
		fp := fingerprintMessagePrefix(messagesRaw, i)
		if fp == "" {
			continue
		}
		key := sessionTraceLookupKey(tokenID, modelName, fp)
		if traceID, found, err := cache.Get(key); err == nil && found && model.IsValidTraceID(traceID) {
			return traceID, true
		}
	}
	return "", false
}

func registerTraceFingerprints(tokenID int, modelName, messagesRaw, traceID string) {
	if messagesRaw == "" || !model.IsValidTraceID(traceID) {
		return
	}
	arr := gjson.Parse(messagesRaw)
	if !arr.IsArray() {
		return
	}
	n := len(arr.Array())
	cache := sessionTraceCacheInstance()
	ttl := sessionTraceTTL()
	for i := 1; i <= n; i++ {
		fp := fingerprintMessagePrefix(messagesRaw, i)
		if fp == "" {
			continue
		}
		key := sessionTraceLookupKey(tokenID, modelName, fp)
		_ = cache.SetWithTTL(key, traceID, ttl)
	}
}

func resolveTraceID(c *gin.Context, tokenID int, modelName, messagesRaw, clientRequest string) string {
	clientTraceID := strings.TrimSpace(c.GetHeader(common.TraceIdKey))
	if model.IsValidTraceID(clientTraceID) {
		return clientTraceID
	}
	if traceID, ok := lookupTraceID(tokenID, modelName, messagesRaw); ok {
		return traceID
	}
	return model.NewTraceID()
}

func IsSessionTraceActive(c *gin.Context) bool {
	return common.GetContextKeyBool(c, constant.ContextKeySessionTraceActive)
}

func SetSessionTraceTurnMeta(c *gin.Context, meta SessionTraceTurnMeta) {
	if c == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeySessionTraceTurnMeta, meta)
}

func getSessionTraceTurnMeta(c *gin.Context) SessionTraceTurnMeta {
	if c == nil {
		return SessionTraceTurnMeta{}
	}
	if meta, ok := c.Get(string(constant.ContextKeySessionTraceTurnMeta)); ok {
		if typed, ok := meta.(SessionTraceTurnMeta); ok {
			return typed
		}
	}
	return SessionTraceTurnMeta{}
}

func getSessionTraceResponseBody(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if writer, ok := c.Get(ginKeySessionTraceWriter); ok {
		if traceWriter, ok := writer.(*common.SessionTraceResponseWriter); ok {
			return traceWriter.ResponseBody()
		}
	}
	return ""
}

// BeginSessionTrace resolves a session trace id and wraps the response writer.
func BeginSessionTrace(c *gin.Context, relayFormat types.RelayFormat, clientRequest string) {
	if !constant.SessionTraceEnabled || c == nil {
		return
	}
	if relayFormat == types.RelayFormatOpenAIRealtime {
		return
	}

	messagesRaw := extractMessagesRaw(clientRequest)
	clientTraceID := strings.TrimSpace(c.GetHeader(common.TraceIdKey))
	if messagesRaw == "" && !model.IsValidTraceID(clientTraceID) {
		return
	}

	tokenID := c.GetInt("token_id")
	modelName := c.GetString("original_model")
	traceID := resolveTraceID(c, tokenID, modelName, messagesRaw, clientRequest)

	common.SetContextKey(c, constant.ContextKeyTraceId, traceID)
	common.SetContextKey(c, constant.ContextKeySessionTraceActive, true)
	common.SetContextKey(c, constant.ContextKeySessionTraceMessagesRaw, messagesRaw)
	c.Header(common.TraceIdKey, traceID)

	maxBytes := constant.SessionTraceMaxMB
	if maxBytes <= 0 {
		maxBytes = 4
	}
	traceWriter := common.NewSessionTraceResponseWriter(c.Writer, maxBytes<<20)
	c.Set(ginKeySessionTraceWriter, traceWriter)
	c.Writer = traceWriter
}

// FinishSessionTraceTurn persists the current relay turn asynchronously.
func FinishSessionTraceTurn(c *gin.Context, status string, errorMessage string) {
	if !constant.SessionTraceEnabled || c == nil {
		return
	}
	if common.GetContextKeyBool(c, constant.ContextKeySessionTraceFinished) {
		return
	}
	if !IsSessionTraceActive(c) {
		return
	}
	common.SetContextKey(c, constant.ContextKeySessionTraceFinished, true)

	traceID := common.GetContextKeyString(c, constant.ContextKeyTraceId)
	if !model.IsValidTraceID(traceID) {
		return
	}

	requestID := c.GetString(common.RequestIdKey)
	userID := c.GetInt("id")
	tokenID := c.GetInt("token_id")
	modelName := c.GetString("original_model")
	channelID := c.GetInt("channel_id")
	isStream := common.GetContextKeyBool(c, constant.ContextKeyIsStream)
	messagesRaw := common.GetContextKeyString(c, constant.ContextKeySessionTraceMessagesRaw)
	turnMeta := getSessionTraceTurnMeta(c)
	clientRequest := readRelayClientRequestBody(c)
	responseBody := getSessionTraceResponseBody(c)

	if status == "" {
		status = "success"
	}
	if status == "error" && errorMessage != "" {
		errorMessage = common.MaskSensitiveInfo(errorMessage)
	}

	gopool.Go(func() {
		recordSessionTraceTurn(
			traceID,
			requestID,
			userID,
			tokenID,
			modelName,
			channelID,
			status,
			errorMessage,
			isStream,
			messagesRaw,
			clientRequest,
			responseBody,
			turnMeta,
		)
	})
}

func readRelayClientRequestBody(c *gin.Context) string {
	storage, err := common.GetBodyStorage(c)
	if err != nil || storage == nil {
		return ""
	}
	body, err := storage.Bytes()
	if err != nil {
		return ""
	}
	return string(body)
}

func recordSessionTraceTurn(
	traceID string,
	requestID string,
	userID int,
	tokenID int,
	modelName string,
	channelID int,
	status string,
	errorMessage string,
	isStream bool,
	messagesRaw string,
	clientRequest string,
	responseBody string,
	turnMeta SessionTraceTurnMeta,
) {
	now := time.Now().Unix()
	session, err := model.GetSessionTrace(traceID)
	if err != nil {
		session = &model.SessionTrace{
			TraceId:        traceID,
			UserId:         userID,
			TokenId:        tokenID,
			ModelName:      modelName,
			TurnCount:      0,
			CreatedAt:      now,
			LastActivityAt: now,
		}
		if createErr := model.UpsertSessionTrace(session); createErr != nil {
			common.SysError("failed to create session trace: " + createErr.Error())
			return
		}
	}

	turnIndex := session.TurnCount + 1
	detail := model.BuildSessionTraceTurnDetailPayload(
		traceID,
		turnIndex,
		requestID,
		clientRequest,
		responseBody,
		isStream,
	)
	if saveErr := model.SaveSessionTraceTurnDetail(detail); saveErr != nil {
		common.SysError("failed to save session trace turn detail: " + saveErr.Error())
	}

	turn := &model.SessionTraceTurn{
		TraceId:          traceID,
		TurnIndex:        turnIndex,
		RequestId:        requestID,
		UserId:           userID,
		TokenId:          tokenID,
		ModelName:        modelName,
		ChannelId:        channelID,
		Status:           status,
		PromptTokens:     turnMeta.PromptTokens,
		CompletionTokens: turnMeta.CompletionTokens,
		IsStream:         isStream,
		ErrorMessage:     errorMessage,
		CreatedAt:        now,
	}
	if insertErr := model.InsertSessionTraceTurn(turn); insertErr != nil {
		common.SysError("failed to insert session trace turn: " + insertErr.Error())
		return
	}

	session.TurnCount = turnIndex
	session.LastActivityAt = now
	session.ModelName = modelName
	if upsertErr := model.UpsertSessionTrace(session); upsertErr != nil {
		common.SysError("failed to update session trace: " + upsertErr.Error())
	}

	registerTraceFingerprints(tokenID, modelName, messagesRaw, traceID)
}

func GetSessionTraceFullView(traceID string) (*SessionTraceFullView, error) {
	session, err := model.GetSessionTrace(traceID)
	if err != nil {
		return nil, err
	}
	turns, err := model.ListSessionTraceTurns(traceID)
	if err != nil {
		return nil, err
	}
	view := &SessionTraceFullView{
		SessionTrace: *session,
		Turns:        make([]SessionTraceTurnDetailView, 0, len(turns)),
	}
	for _, turn := range turns {
		item := SessionTraceTurnDetailView{SessionTraceTurn: turn}
		if detail, loadErr := model.LoadSessionTraceTurnDetail(traceID, turn.TurnIndex); loadErr == nil {
			item.Detail = detail
		}
		view.Turns = append(view.Turns, item)
	}
	return view, nil
}
