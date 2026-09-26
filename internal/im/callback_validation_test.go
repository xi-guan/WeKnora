package im_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/im"
	"github.com/Tencent/WeKnora/internal/im/feishu"
	"github.com/Tencent/WeKnora/internal/im/mattermost"
	"github.com/Tencent/WeKnora/internal/im/slack"
	"github.com/Tencent/WeKnora/internal/im/telegram"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMissingWebhookSecretsFailClosed(t *testing.T) {
	for _, adapter := range []im.Adapter{
		&feishu.Adapter{}, &mattermost.Adapter{},
		&slack.Adapter{}, &telegram.Adapter{},
	} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("POST", "/callback", nil)
		require.Error(t, adapter.VerifyCallback(ctx), "%T", adapter)
	}
}

func TestSlackRejectsAttackerDownload(t *testing.T) {
	// A nil API client also proves rejection happens before any SDK/network call.
	adapter := &slack.Adapter{}
	for _, target := range []string{
		"https://evil.example/download", "https://files.slack.com.evil.example/",
		"http://files.slack.com/file", "https://files.slack.com:444/file",
	} {
		_, _, err := adapter.DownloadFile(context.Background(), &im.IncomingMessage{
			FileKey: "file", Extra: map[string]string{"url_private_download": target},
		})
		require.Error(t, err)
	}
}
