package notion

import (
	"strings"

	"github.com/jomei/notionapi"
)

// RenderBlocks converts the Notion block subset used by dictionary pages into
// newline-separated plain text. Unsupported blocks are ignored.
func RenderBlocks(blocks []notionapi.Block) string {
	lines := renderBlockLines(blocks)
	return strings.Join(lines, "\n")
}

func renderBlockLines(blocks []notionapi.Block) []string {
	lines := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block == nil {
			continue
		}
		if text, ok := renderBlockText(block); ok {
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				lines = append(lines, trimmed)
			}
		}
		lines = append(lines, renderBlockLines(blockChildren(block))...)
	}
	return lines
}

func renderBlockText(block notionapi.Block) (string, bool) {
	switch block.GetType() {
	case notionapi.BlockTypeParagraph,
		notionapi.BlockTypeBulletedListItem,
		notionapi.BlockTypeNumberedListItem:
		return block.GetRichTextString(), true
	default:
		return "", false
	}
}

func blockChildren(block notionapi.Block) []notionapi.Block {
	switch b := block.(type) {
	case *notionapi.ParagraphBlock:
		return b.Paragraph.Children
	case notionapi.ParagraphBlock:
		return b.Paragraph.Children
	case *notionapi.BulletedListItemBlock:
		return b.BulletedListItem.Children
	case notionapi.BulletedListItemBlock:
		return b.BulletedListItem.Children
	case *notionapi.NumberedListItemBlock:
		return b.NumberedListItem.Children
	case notionapi.NumberedListItemBlock:
		return b.NumberedListItem.Children
	default:
		return nil
	}
}
