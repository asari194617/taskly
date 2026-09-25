package gqlMutation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/authsync"
	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"gorm.io/gorm"
)

var errUserProjectionConflict = errors.New("auth user projection conflict")

func (h *Handler) createAdminUser(ctx context.Context, email, displayName, notificationLang string) (*domain.User, bool, error) {
	email = strings.TrimSpace(email)
	if !authsync.UsersEnabled() {
		record := &domain.User{Email: email, DisplayName: displayName, NotificationLang: notificationLang}
		if err := h.db.WithContext(ctx).Create(record).Error; err != nil {
			return nil, false, err
		}
		return record, true, nil
	}
	email = strings.ToLower(email)
	displayName = strings.TrimSpace(displayName)
	result, err := authsync.ProvisionUser(ctx, authsync.UserProvision{Email: email, DisplayName: displayName, NotificationLang: notificationLang})
	if err != nil {
		return nil, false, err
	}
	var existing domain.User
	err = h.db.WithContext(ctx).Where("id = ? OR email = ?", result.ID, email).First(&existing).Error
	if err == nil {
		if existing.ID == result.ID && strings.EqualFold(existing.Email, email) {
			return &existing, false, nil
		}
		return nil, false, errUserProjectionConflict
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, fmt.Errorf("%w: inspect app projection", authsync.ErrUserSync)
	}
	record := &domain.User{ID: result.ID, Email: email, DisplayName: displayName, NotificationLang: notificationLang}
	if err := h.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, false, fmt.Errorf("%w: create app projection", authsync.ErrUserSync)
	}
	return record, true, nil
}

func (h *Handler) updateAdminUser(ctx context.Context, userID uint, updates map[string]any) error {
	if !authsync.UsersEnabled() {
		return h.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", userID).Updates(updates).Error
	}
	var current domain.User
	if err := h.db.WithContext(ctx).First(&current, userID).Error; err != nil {
		return err
	}
	input := authsync.UserUpdate{}
	if value, ok := updates["email"].(string); ok {
		value = strings.ToLower(strings.TrimSpace(value))
		updates["email"] = value
		input.Email = &value
	}
	if value, ok := updates["display_name"].(string); ok {
		value = strings.TrimSpace(value)
		updates["display_name"] = value
		input.DisplayName = &value
	}
	if value, ok := updates["notification_lang"].(string); ok {
		input.NotificationLang = &value
	}
	if value, ok := updates["email_code_allowed"].(bool); ok {
		input.EmailCodeAllowed = &value
	}
	if err := authsync.UpdateUser(ctx, userID, input); err != nil {
		return err
	}
	emailChanged := input.Email != nil && !strings.EqualFold(current.Email, *input.Email)
	if err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&domain.User{}).Where("id = ?", userID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if emailChanged {
			return tx.Where("user_id = ?", userID).Delete(&domain.Session{}).Error
		}
		return nil
	}); err != nil {
		return fmt.Errorf("%w: update app projection", authsync.ErrUserSync)
	}
	if emailChanged {
		utils.ClearUserSessionCache(ctx, userID, h.rdb)
	}
	return nil
}
