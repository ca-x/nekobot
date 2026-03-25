package parse

import (
	"testing"

	"nekobot/pkg/wechat/types"
)

func TestExtractTextIncludesQuotedPrefix(t *testing.T) {
	t.Parallel()

	msg := &types.WeixinMessage{
		ItemList: []types.MessageItem{
			{
				TextItem: &types.TextItem{Text: "reply"},
				RefMsg:   &types.RefMsg{Title: "older message"},
			},
		},
	}

	got := ExtractText(msg)
	want := "[引用: older message]\nreply"
	if got != want {
		t.Fatalf("unexpected text: %q", got)
	}
}

func TestExtractMediaFallsBackToRefMessage(t *testing.T) {
	t.Parallel()

	msg := &types.WeixinMessage{
		ItemList: []types.MessageItem{
			{
				TextItem: &types.TextItem{Text: "see quote"},
				RefMsg: &types.RefMsg{
					MessageItem: &types.MessageItem{
						Type:      types.ItemTypeFile,
						FileItem:  &types.FileItem{FileName: "report.pdf"},
						TextItem:  nil,
						ImageItem: nil,
					},
				},
			},
		},
	}

	got := ExtractMedia(msg)
	if got == nil || got.FileItem == nil || got.FileItem.FileName != "report.pdf" {
		t.Fatalf("unexpected media item: %#v", got)
	}
}
