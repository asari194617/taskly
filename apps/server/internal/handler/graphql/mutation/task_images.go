package gqlMutation

import (
	"context"
	"fmt"
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"github.com/asari194617/taskly/apps/server/internal/taskimage"
	"github.com/asari194617/taskly/apps/server/internal/utils"
	"gorm.io/gorm"
)

func taskImageURL(ctx context.Context, key *string) any {
	if key == nil || strings.TrimSpace(*key) == "" {
		return nil
	}
	images, err := taskimage.NewFromEnv()
	if err != nil {
		return nil
	}
	url, err := images.PresignView(ctx, *key)
	if err != nil {
		return nil
	}
	return url
}

func deleteTaskImage(ctx context.Context, key *string) {
	if key == nil || strings.TrimSpace(*key) == "" {
		return
	}
	images, err := taskimage.NewFromEnv()
	if err != nil {
		utils.CaptureError(err)
		return
	}
	if err := images.Delete(ctx, *key); err != nil {
		utils.CaptureError(err)
	}
}

func parseTaskImageKeys(value any) ([]string, error) {
	var values []any
	switch typed := value.(type) {
	case []any:
		values = typed
	case []string:
		values = make([]any, 0, len(typed))
		for _, key := range typed {
			values = append(values, key)
		}
	default:
		return nil, fmt.Errorf("invalid image keys")
	}
	keys := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		key, ok := value.(string)
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("invalid image key")
		}
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate image key")
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	if len(keys) > taskimage.MaxTaskImages {
		return nil, fmt.Errorf("too many images")
	}
	return keys, nil
}

func validateTaskImageKeys(ctx context.Context, userID uint, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	images, err := taskimage.NewFromEnv()
	if err != nil {
		return fmt.Errorf("image upload unavailable")
	}
	for _, key := range keys {
		if err := images.ValidateUploaded(ctx, userID, key); err != nil {
			return fmt.Errorf("invalid image upload")
		}
	}
	return nil
}

func taskImageFields(ctx context.Context, keys []string) (any, []string) {
	if len(keys) == 0 {
		return nil, []string{}
	}
	images, err := taskimage.NewFromEnv()
	if err != nil {
		return nil, []string{}
	}
	urls := make([]string, 0, len(keys))
	for _, key := range keys {
		url, err := images.PresignView(ctx, key)
		if err != nil {
			return nil, []string{}
		}
		urls = append(urls, url)
	}
	if len(urls) == 0 {
		return nil, urls
	}
	return urls[0], urls
}

func taskKeys(db *gorm.DB, task domain.Task) []string {
	keys, err := taskimage.KeysForTask(db, task.ID)
	if err == nil && len(keys) > 0 {
		return keys
	}
	if task.ImageKey != nil && strings.TrimSpace(*task.ImageKey) != "" {
		return []string{*task.ImageKey}
	}
	return nil
}

func deleteTaskImages(ctx context.Context, keys []string) {
	for _, key := range keys {
		key := key
		deleteTaskImage(ctx, &key)
	}
}

func removedTaskImageKeys(before, after []string) []string {
	retained := make(map[string]struct{}, len(after))
	for _, key := range after {
		retained[key] = struct{}{}
	}
	removed := make([]string, 0)
	for _, key := range before {
		if _, ok := retained[key]; !ok {
			removed = append(removed, key)
		}
	}
	return removed
}
