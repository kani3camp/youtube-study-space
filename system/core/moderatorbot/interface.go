package moderatorbot

import (
	"context"
)

type MessageBot interface {
	SendMessage(ctx context.Context, message string) error
	SendMessageWithError(ctx context.Context, message string, err error) error
}
