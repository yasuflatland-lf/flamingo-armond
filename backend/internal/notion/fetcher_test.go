package notion

import (
	"context"
	"errors"
	"testing"

	"github.com/jomei/notionapi"
)

type stubPageService struct {
	pages map[notionapi.PageID]*notionapi.Page
	err   error
	calls []notionapi.PageID
}

func (s *stubPageService) Get(_ context.Context, id notionapi.PageID) (*notionapi.Page, error) {
	s.calls = append(s.calls, id)
	if s.err != nil {
		return nil, s.err
	}
	return s.pages[id], nil
}

type stubBlockService struct {
	responses map[notionapi.BlockID][]*notionapi.GetChildrenResponse
	err       error
	calls     []notionapi.BlockID
}

func (s *stubBlockService) GetChildren(_ context.Context, id notionapi.BlockID, _ *notionapi.Pagination) (*notionapi.GetChildrenResponse, error) {
	s.calls = append(s.calls, id)
	if s.err != nil {
		return nil, s.err
	}
	resp := s.responses[id][0]
	s.responses[id] = s.responses[id][1:]
	return resp, nil
}

func TestFetcherFetchPages(t *testing.T) {
	t.Parallel()

	pages := &stubPageService{pages: map[notionapi.PageID]*notionapi.Page{
		"page-1": {
			ID: "page-1",
			Properties: notionapi.Properties{
				"Name": &notionapi.TitleProperty{
					Title: []notionapi.RichText{{PlainText: "English"}},
				},
			},
		},
	}}
	blocks := &stubBlockService{responses: map[notionapi.BlockID][]*notionapi.GetChildrenResponse{
		"page-1": {
			{
				Results: []notionapi.Block{
					&notionapi.ParagraphBlock{
						BasicBlock: notionapi.BasicBlock{Type: notionapi.BlockTypeParagraph},
						Paragraph: notionapi.Paragraph{
							RichText: []notionapi.RichText{{PlainText: "ulcers \u6f70\u760d"}},
						},
					},
				},
				HasMore:    true,
				NextCursor: "next",
			},
			{
				Results: []notionapi.Block{
					&notionapi.ParagraphBlock{
						BasicBlock: notionapi.BasicBlock{Type: notionapi.BlockTypeParagraph},
						Paragraph: notionapi.Paragraph{
							RichText: []notionapi.RichText{{PlainText: "wayside \u8def\u7aef"}},
						},
					},
				},
			},
		},
	}}

	out, err := NewFetcherFromServices(pages, blocks).FetchPages(context.Background(), []string{" page-1 "})
	if err != nil {
		t.Fatalf("FetchPages: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].ID != "page-1" || out[0].Title != "English" {
		t.Fatalf("page metadata = %+v", out[0])
	}
	wantText := "ulcers \u6f70\u760d\nwayside \u8def\u7aef"
	if out[0].Text != wantText {
		t.Fatalf("Text = %q, want %q", out[0].Text, wantText)
	}
	if len(blocks.calls) != 2 {
		t.Fatalf("block calls = %d, want 2", len(blocks.calls))
	}
}

func TestFetcherAbortOnFetchError(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	pages := &stubPageService{err: boom}
	blocks := &stubBlockService{}

	_, err := NewFetcherFromServices(pages, blocks).FetchPages(context.Background(), []string{"page-1", "page-2"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if len(pages.calls) != 1 {
		t.Fatalf("page calls = %d, want fail-fast after 1", len(pages.calls))
	}
	if len(blocks.calls) != 0 {
		t.Fatalf("block calls = %d, want 0", len(blocks.calls))
	}
}
