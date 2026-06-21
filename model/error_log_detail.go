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

const errorDetailIDJSONKey = "error_detail_id"

var errorDetailIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ErrorLogDetail stores the full error context for a relay failure, including
// the client request payload and upstream response body.
type ErrorLogDetail struct {
	DetailID          string `json:"detail_id"`
	RequestID         string `json:"request_id,omitempty"`
	UpstreamRequestID string `json:"upstream_request_id,omitempty"`
	UserID            int    `json:"user_id"`
	LogID             int    `json:"log_id,omitempty"`
	CreatedAt         int64  `json:"created_at"`
	ErrorMessage      string `json:"error_message"`
	ErrorType         string `json:"error_type,omitempty"`
	ErrorCode         string `json:"error_code,omitempty"`
	StatusCode        int    `json:"status_code,omitempty"`
	ClientRequest     string `json:"client_request,omitempty"`
	UpstreamResponse  string `json:"upstream_response,omitempty"`
	RequestPath       string `json:"request_path,omitempty"`
	ModelName         string `json:"model_name,omitempty"`
	ChannelID         int    `json:"channel_id,omitempty"`
	ChannelName       string `json:"channel_name,omitempty"`
	IsStream          bool   `json:"is_stream,omitempty"`
	Truncated         bool   `json:"truncated,omitempty"`
}

func NewErrorDetailID() string {
	return uuid.NewString()
}

func IsValidErrorDetailID(id string) bool {
	return errorDetailIDPattern.MatchString(id)
}

func errorLogDetailMaxBytes() int {
	maxMB := constant.ErrorLogDetailMaxMB
	if maxMB <= 0 {
		maxMB = 4
	}
	return maxMB << 20
}

func errorLogDetailDir() string {
	if constant.ErrorLogDetailDir != "" {
		return constant.ErrorLogDetailDir
	}
	return "./data/error-log-details"
}

func ensureErrorLogDetailDir() error {
	dir := errorLogDetailDir()
	if _, err := os.Stat(dir); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(dir, 0755)
}

func errorLogDetailPath(detailID string) string {
	return filepath.Join(errorLogDetailDir(), detailID+".json")
}

func truncateForErrorDetail(value string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value, false
	}
	truncated := value[:maxBytes]
	return truncated, true
}

func maskAndLimitDetailText(value string, maxBytes int) (string, bool) {
	masked := common.MaskSensitiveInfo(value)
	return truncateForErrorDetail(masked, maxBytes)
}

// SaveErrorLogDetail persists the detail payload as JSON on disk.
func SaveErrorLogDetail(detail *ErrorLogDetail) error {
	if detail == nil {
		return errors.New("error log detail is nil")
	}
	if !IsValidErrorDetailID(detail.DetailID) {
		return errors.New("invalid error detail id")
	}
	if err := ensureErrorLogDetailDir(); err != nil {
		return err
	}
	data, err := common.Marshal(detail)
	if err != nil {
		return err
	}
	path := errorLogDetailPath(detail.DetailID)
	return os.WriteFile(path, data, 0644)
}

// LoadErrorLogDetail reads a saved detail payload from disk.
func LoadErrorLogDetail(detailID string) (*ErrorLogDetail, error) {
	if !IsValidErrorDetailID(detailID) {
		return nil, errors.New("invalid error detail id")
	}
	data, err := os.ReadFile(errorLogDetailPath(detailID))
	if err != nil {
		return nil, err
	}
	var detail ErrorLogDetail
	if err := common.Unmarshal(data, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// DeleteErrorLogDetail removes a saved detail file from disk.
func DeleteErrorLogDetail(detailID string) error {
	if !IsValidErrorDetailID(detailID) {
		return nil
	}
	err := os.Remove(errorLogDetailPath(detailID))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// FindLogByErrorDetailID finds the log row associated with a saved detail id.
func FindLogByErrorDetailID(detailID string) (*Log, error) {
	if !IsValidErrorDetailID(detailID) {
		return nil, errors.New("invalid error detail id")
	}
	pattern := fmt.Sprintf(`%%"%s":"%s"%%`, errorDetailIDJSONKey, detailID)
	var log Log
	err := LOG_DB.Where("type = ? AND other LIKE ?", LogTypeError, pattern).Order("id desc").First(&log).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("log not found")
		}
		return nil, err
	}
	return &log, nil
}

func extractErrorDetailID(other string) string {
	if other == "" {
		return ""
	}
	otherMap, err := common.StrToMap(other)
	if err != nil || otherMap == nil {
		return ""
	}
	raw, ok := otherMap[errorDetailIDJSONKey]
	if !ok || raw == nil {
		return ""
	}
	detailID, ok := raw.(string)
	if !ok || !IsValidErrorDetailID(detailID) {
		return ""
	}
	return detailID
}

// CleanupErrorLogDetailsBefore deletes detail files referenced by error logs
// older than targetTimestamp.
func CleanupErrorLogDetailsBefore(targetTimestamp int64, limit int) (int, error) {
	if targetTimestamp <= 0 || limit <= 0 {
		return 0, nil
	}
	var logs []Log
	err := LOG_DB.Select("id", "other").
		Where("type = ? AND created_at < ?", LogTypeError, targetTimestamp).
		Limit(limit).
		Find(&logs).Error
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, log := range logs {
		detailID := extractErrorDetailID(log.Other)
		if detailID == "" {
			continue
		}
		if err := DeleteErrorLogDetail(detailID); err != nil {
			common.SysLog("failed to delete error log detail file: " + err.Error())
			continue
		}
		deleted++
	}
	return deleted, nil
}

// BuildErrorLogDetailPayload prepares a detail payload from relay error context.
func BuildErrorLogDetailPayload(
	detailID string,
	userID int,
	errorMessage string,
	errorType string,
	errorCode string,
	statusCode int,
	clientRequest string,
	upstreamResponse string,
	requestPath string,
	modelName string,
	channelID int,
	channelName string,
	isStream bool,
	requestID string,
	upstreamRequestID string,
) *ErrorLogDetail {
	maxBytes := errorLogDetailMaxBytes() / 2
	truncated := false

	clientRequest, clientTruncated := maskAndLimitDetailText(clientRequest, maxBytes)
	upstreamResponse, upstreamTruncated := maskAndLimitDetailText(upstreamResponse, maxBytes)
	errorMessage, errorTruncated := maskAndLimitDetailText(errorMessage, maxBytes)
	truncated = clientTruncated || upstreamTruncated || errorTruncated

	return &ErrorLogDetail{
		DetailID:          detailID,
		RequestID:         strings.TrimSpace(requestID),
		UpstreamRequestID: strings.TrimSpace(upstreamRequestID),
		UserID:            userID,
		CreatedAt:         common.GetTimestamp(),
		ErrorMessage:      errorMessage,
		ErrorType:         errorType,
		ErrorCode:         errorCode,
		StatusCode:        statusCode,
		ClientRequest:     clientRequest,
		UpstreamResponse:  upstreamResponse,
		RequestPath:       requestPath,
		ModelName:         modelName,
		ChannelID:         channelID,
		ChannelName:       channelName,
		IsStream:          isStream,
		Truncated:         truncated,
	}
}
