package notion

import (
	"context"
	"errors"
	"testing"

	"github.com/jomei/notionapi"
)

type stubBlockAppendService struct {
	response *notionapi.AppendBlockChildrenResponse
	err      error
	calls    []*notionapi.AppendBlockChildrenRequest
	ids      []notionapi.BlockID
}

func (s *stubBlockAppendService) AppendChildren(_ context.Context, id notionapi.BlockID, req *notionapi.AppendBlockChildrenRequest) (*notionapi.AppendBlockChildrenResponse, error) {
	s.ids = append(s.ids, id)
	s.calls = append(s.calls, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.response, nil
}

func TestWriterAppendParagraphHappyPath(t *testing.T) {
	t.Parallel()

	stub := &stubBlockAppendService{
		response: &notionapi.AppendBlockChildrenResponse{},
	}
	w := newWriterFromService(stub)

	err := w.AppendParagraph(context.Background(), "page-abc", "front back")
	if err != nil {
		t.Fatalf("AppendParagraph: %v", err)
	}
	if len(stub.calls) != 1 {
		t.Fatalf("AppendChildren calls = %d, want 1", len(stub.calls))
	}
	if stub.ids[0] != notionapi.BlockID("page-abc") {
		t.Fatalf("page id = %q, want %q", stub.ids[0], "page-abc")
	}

	req := stub.calls[0]
	if len(req.Children) != 1 {
		t.Fatalf("children count = %d, want 1", len(req.Children))
	}
	para, ok := req.Children[0].(notionapi.ParagraphBlock)
	if !ok {
		t.Fatalf("child type = %T, want ParagraphBlock", req.Children[0])
	}
	if len(para.Paragraph.RichText) != 1 {
		t.Fatalf("rich text count = %d, want 1", len(para.Paragraph.RichText))
	}
	rt := para.Paragraph.RichText[0]
	if rt.Text == nil {
		t.Fatal("RichText.Text is nil")
	}
	if rt.Text.Content != "front back" {
		t.Fatalf("Text.Content = %q, want %q", rt.Text.Content, "front back")
	}
}

func TestWriterAppendParagraphErrorPath(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("notion api down")
	stub := &stubBlockAppendService{
		err: sentinel,
	}
	w := newWriterFromService(stub)

	err := w.AppendParagraph(context.Background(), "page-xyz", "some text")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want to wrap sentinel %v", err, sentinel)
	}
}
