package model

import (
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// tokenVerificationOptionKey 是存储在 options 表中的键名。
// 使用 JSON map（token_id -> true）保存所有启用了 X-Verification-Code 校验的令牌。
// 这样可以在不修改数据库结构的前提下持久化令牌粒度的验证设置。
const tokenVerificationOptionKey = "TokenVerificationCodeMap"

var (
	tokenVerificationMap   = make(map[int]bool)
	tokenVerificationMutex sync.RWMutex
)

// LoadTokenVerificationSettings 在系统启动时从 options 表加载验证设置到内存。
// 必须在 InitDB / InitOptionMap 之后调用。
func LoadTokenVerificationSettings() {
	tokenVerificationMutex.Lock()
	defer tokenVerificationMutex.Unlock()

	var option Option
	if err := DB.Where(Option{Key: tokenVerificationOptionKey}).First(&option).Error; err != nil {
		// 首次使用，options 表中尚无记录，保持空 map 即可
		return
	}
	if option.Value == "" {
		return
	}
	if err := common.Unmarshal([]byte(option.Value), &tokenVerificationMap); err != nil {
		common.SysError("failed to unmarshal token verification map: " + err.Error())
		tokenVerificationMap = make(map[int]bool)
	}
}

// saveTokenVerificationMap 将内存中的验证设置持久化到 options 表。
func saveTokenVerificationMap() error {
	tokenVerificationMutex.RLock()
	data, err := common.Marshal(tokenVerificationMap)
	tokenVerificationMutex.RUnlock()
	if err != nil {
		return err
	}

	option := Option{Key: tokenVerificationOptionKey}
	DB.FirstOrCreate(&option, Option{Key: tokenVerificationOptionKey})
	option.Value = string(data)
	return DB.Save(&option).Error
}

// SetTokenVerificationEnabled 设置指定令牌的验证码校验开关并持久化。
func SetTokenVerificationEnabled(tokenId int, enabled bool) error {
	tokenVerificationMutex.Lock()
	if enabled {
		tokenVerificationMap[tokenId] = true
	} else {
		delete(tokenVerificationMap, tokenId)
	}
	tokenVerificationMutex.Unlock()

	return saveTokenVerificationMap()
}

// IsTokenVerificationEnabled 返回指定令牌是否启用了 X-Verification-Code 校验。
func IsTokenVerificationEnabled(tokenId int) bool {
	tokenVerificationMutex.RLock()
	defer tokenVerificationMutex.RUnlock()
	return tokenVerificationMap[tokenId]
}

// DeleteTokenVerificationSetting 删除指定令牌的验证设置（用于令牌删除时清理）。
func DeleteTokenVerificationSetting(tokenId int) error {
	tokenVerificationMutex.Lock()
	delete(tokenVerificationMap, tokenId)
	tokenVerificationMutex.Unlock()

	return saveTokenVerificationMap()
}

// PopulateTokenVerificationFields 批量填充令牌的 VerificationCodeEnabled 字段。
// 用于从数据库加载令牌后、返回给前端或缓存之前补充该字段值。
func PopulateTokenVerificationFields(tokens ...*Token) {
	tokenVerificationMutex.RLock()
	defer tokenVerificationMutex.RUnlock()
	for _, token := range tokens {
		if token == nil {
			continue
		}
		token.VerificationCodeEnabled = tokenVerificationMap[token.Id]
	}
}
