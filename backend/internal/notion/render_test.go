package notion

import (
	"encoding/json"
	"testing"

	"github.com/jomei/notionapi"
)

func TestRenderBlocksSupportedPlainText(t *testing.T) {
	t.Parallel()

	var blocks notionapi.Blocks
	input := []byte(`[
		{
			"object":"block",
			"id":"p1",
			"type":"paragraph",
			"paragraph":{"rich_text":[
				{"plain_text":"ulcers "},
				{"plain_text":"\u6f70\u760d"}
			]}
		},
		{
			"object":"block",
			"id":"b1",
			"type":"bulleted_list_item",
			"bulleted_list_item":{"rich_text":[{"plain_text":"wayside \u8def\u7aef"}]}
		},
		{
			"object":"block",
			"id":"n1",
			"type":"numbered_list_item",
			"numbered_list_item":{"rich_text":[{"plain_text":"hardwired \u6839\u6df1\u3044"}]}
		},
		{
			"object":"block",
			"id":"d1",
			"type":"divider",
			"divider":{}
		}
	]`)
	if err := json.Unmarshal(input, &blocks); err != nil {
		t.Fatalf("unmarshal blocks: %v", err)
	}

	got := RenderBlocks(blocks)
	want := "ulcers \u6f70\u760d\nwayside \u8def\u7aef\nhardwired \u6839\u6df1\u3044"
	if got != want {
		t.Fatalf("RenderBlocks() = %q, want %q", got, want)
	}
}

func TestRenderBlocksSkipsNilEntries(t *testing.T) {
	t.Parallel()

	blocks := []notionapi.Block{
		nil,
		&notionapi.ParagraphBlock{
			BasicBlock: notionapi.BasicBlock{Type: notionapi.BlockTypeParagraph},
			Paragraph: notionapi.Paragraph{
				RichText: []notionapi.RichText{{PlainText: "after-nil"}},
			},
		},
		nil,
	}

	got := RenderBlocks(blocks)
	want := "after-nil"
	if got != want {
		t.Fatalf("RenderBlocks() = %q, want %q", got, want)
	}
}

func TestRenderBlocksUnsupportedParentDropsChildren(t *testing.T) {
	t.Parallel()

	// A ToggleBlock is unsupported by both renderBlockText and blockChildren,
	// so its supported child Paragraph is silently dropped. Pin this contract
	// so a future change that decides to render toggle children does so
	// deliberately rather than as a side effect.
	blocks := []notionapi.Block{
		&notionapi.ToggleBlock{
			BasicBlock: notionapi.BasicBlock{Type: notionapi.BlockTypeToggle},
			Toggle: notionapi.Toggle{
				RichText: []notionapi.RichText{{PlainText: "toggle-header"}},
				Children: []notionapi.Block{
					&notionapi.ParagraphBlock{
						BasicBlock: notionapi.BasicBlock{Type: notionapi.BlockTypeParagraph},
						Paragraph: notionapi.Paragraph{
							RichText: []notionapi.RichText{{PlainText: "child-of-toggle"}},
						},
					},
				},
			},
		},
	}

	got := RenderBlocks(blocks)
	if got != "" {
		t.Fatalf("RenderBlocks() = %q, want empty (unsupported toggle drops both header and children)", got)
	}
}

func TestRenderBlocksNestedChildren(t *testing.T) {
	t.Parallel()

	blocks := []notionapi.Block{
		&notionapi.BulletedListItemBlock{
			BasicBlock: notionapi.BasicBlock{Type: notionapi.BlockTypeBulletedListItem},
			BulletedListItem: notionapi.ListItem{
				RichText: []notionapi.RichText{{PlainText: "parent \u89aa"}},
				Children: []notionapi.Block{
					&notionapi.ParagraphBlock{
						BasicBlock: notionapi.BasicBlock{Type: notionapi.BlockTypeParagraph},
						Paragraph: notionapi.Paragraph{
							RichText: []notionapi.RichText{{PlainText: "child \u5b50"}},
						},
					},
				},
			},
		},
	}

	got := RenderBlocks(blocks)
	want := "parent \u89aa\nchild \u5b50"
	if got != want {
		t.Fatalf("RenderBlocks() = %q, want %q", got, want)
	}
}
