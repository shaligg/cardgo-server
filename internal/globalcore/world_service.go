package globalcore

import "context"

type WorldService interface {
	PublishAnnouncement(ctx context.Context, operatorUID string, content string, reqID string) error
}
