package gqlMutation

import (
	"context"
	"strings"

	"github.com/asari194617/taskly/apps/server/internal/taskimage"
	"github.com/asari194617/taskly/apps/server/internal/utils"
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
