package messaging

import (
	"context"

	"nekobot/pkg/wechat/client"
)

// SendErrorNotice sends a text message to the user on a best-effort basis.
func SendErrorNotice(ctx context.Context, c *client.Client, toUserID, text, contextToken string) {
	clientID := NewClientID()
	_ = SendText(ctx, c, toUserID, text, contextToken, clientID)
}
