package globalcore

import "context"

type GuildService interface {
	ApplyJoin(ctx context.Context, uid string, guildID string, reqID string) error
	ApproveJoin(ctx context.Context, operatorUID string, guildID string, targetUID string, reqID string) error
}
