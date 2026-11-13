package utils

import (
	"errors"
	"fmt"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"gorm.io/gorm"
	"strconv"
	"strings"
)

// small helpers
func ToUint(s string) uint {
	v, err := strconv.ParseUint(s, 10, strconv.IntSize)
	if err != nil {
		return 0
	}
	return uint(v)
}

// (removed obsolete TLS wrappers; smtpSend uses crypto/tls directly)

// Authorization and permission helpers
func UserHasRole(userID uint, role string, DB *gorm.DB) (bool, error) {
	var cnt int64
	if err := DB.Model(&domain.RoleUser{}).Where("user_id = ? AND role = ?", userID, role).Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func ApiKeyLimitForUser(userID uint, DB *gorm.DB) (int, error) {
	if ok, err := UserHasRole(userID, "admin", DB); err != nil {
		return 0, err
	} else if ok {
		return -1, nil
	}
	if ok, err := UserHasRole(userID, "manager", DB); err != nil {
		return 0, err
	} else if ok {
		return 100, nil
	}
	var p domain.Permission
	if err := DB.Where("user_id = ? AND target = ?", userID, "api_key_limit").Order("id DESC").First(&p).Error; err == nil && p.Value != nil {
		var v int
		fmt.Sscanf(*p.Value, "%d", &v)
		if v >= 0 {
			return v, nil
		}
	}
	return 10, nil
}

func CheckPermission(userID uint, target, action string, ownerID *uint, DB *gorm.DB) (bool, error) {
	if ok, err := UserHasRole(userID, "admin", DB); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}
	if ok, err := UserHasRole(userID, "manager", DB); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}
	var p domain.Permission
	if err := DB.Where("user_id = ? AND target = ? AND method = ?", userID, target, action).Order("id DESC").First(&p).Error; err == nil {
		if p.Value == nil || *p.Value == "" || strings.EqualFold(*p.Value, "all") {
			return true, nil
		}
		if strings.EqualFold(*p.Value, "own") {
			if ownerID != nil && *ownerID == userID {
				return true, nil
			}
			return false, nil
		}
		return false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	switch strings.ToLower(action) {
	case "read":
		return true, nil
	case "create":
		return true, nil
	case "update":
		if ownerID != nil && *ownerID == userID {
			return true, nil
		}
		return false, nil
	case "delete":
		return false, nil
	default:
		return false, nil
	}
}
