package notion

import (
	"context"

	"github.com/jomei/notionapi"
	"github.com/rotisserie/eris"
)

type blockAppendService interface {
	AppendChildren(ctx context.Context, id notionapi.BlockID, req *notionapi.AppendBlockChildrenRequest) (*notionapi.AppendBlockChildrenResponse, error)
}

type Writer struct {
	blocks blockAppendService
}

func NewWriter(token string, cfg RetryConfig) *Writer {
	client := notionapi.NewClient(
		notionapi.Token(token),
		notionapi.WithHTTPClient(NewHTTPClient(cfg)),
		notionapi.WithRetry(1),
	)
	return &Writer{blocks: client.Block}
}

func newWriterFromService(blocks blockAppendService) *Writer {
	return &Writer{blocks: blocks}
}

func (w *Writer) AppendParagraph(ctx context.Context, pageID, text string) error {
	block := notionapi.ParagraphBlock{
		BasicBlock: notionapi.BasicBlock{
			Object: notionapi.ObjectTypeBlock,
			Type:   notionapi.BlockTypeParagraph,
		},
		Paragraph: notionapi.Paragraph{
			RichText: []notionapi.RichText{
				{
					Type: notionapi.ObjectTypeText,
					Text: &notionapi.Text{Content: text},
				},
			},
		},
	}
	req := &notionapi.AppendBlockChildrenRequest{Children: []notionapi.Block{block}}
	if _, err := w.blocks.AppendChildren(ctx, notionapi.BlockID(pageID), req); err != nil {
		return eris.Wrapf(err, "notion: append paragraph (page_id=%s)", pageID)
	}
	return nil
}
