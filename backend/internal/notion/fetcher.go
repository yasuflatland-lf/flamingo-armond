package notion

import (
	"context"
	"strings"

	"github.com/jomei/notionapi"
	"github.com/rotisserie/eris"
)

type Page struct {
	ID    string
	Title string
	Text  string
}

type Fetcher interface {
	FetchPages(ctx context.Context, pageIDs []string) ([]Page, error)
}

type pageService interface {
	Get(context.Context, notionapi.PageID) (*notionapi.Page, error)
}

type blockService interface {
	GetChildren(context.Context, notionapi.BlockID, *notionapi.Pagination) (*notionapi.GetChildrenResponse, error)
}

type APIClientFetcher struct {
	pages  pageService
	blocks blockService
}

func NewFetcher(token string, cfg RetryConfig) *APIClientFetcher {
	client := notionapi.NewClient(
		notionapi.Token(token),
		notionapi.WithHTTPClient(NewHTTPClient(cfg)),
		notionapi.WithRetry(1),
	)
	return &APIClientFetcher{pages: client.Page, blocks: client.Block}
}

func NewFetcherFromServices(pages pageService, blocks blockService) *APIClientFetcher {
	return &APIClientFetcher{pages: pages, blocks: blocks}
}

func (f *APIClientFetcher) FetchPages(ctx context.Context, pageIDs []string) ([]Page, error) {
	if len(pageIDs) == 0 {
		return []Page{}, nil
	}
	if f == nil || f.pages == nil || f.blocks == nil {
		return nil, eris.New("notion: fetcher dependencies are not configured")
	}

	out := make([]Page, 0, len(pageIDs))
	for _, rawID := range pageIDs {
		pageID := strings.TrimSpace(rawID)
		if pageID == "" {
			continue
		}
		page, err := f.pages.Get(ctx, notionapi.PageID(pageID))
		if err != nil {
			return nil, eris.Wrapf(err, "notion: fetch page %s", pageID)
		}
		blocks, err := f.fetchBlockChildren(ctx, notionapi.BlockID(pageID))
		if err != nil {
			return nil, eris.Wrapf(err, "notion: fetch block children %s", pageID)
		}
		out = append(out, Page{
			ID:    pageID,
			Title: pageTitle(page),
			Text:  RenderBlocks(blocks),
		})
	}
	return out, nil
}

func (f *APIClientFetcher) fetchBlockChildren(ctx context.Context, blockID notionapi.BlockID) ([]notionapi.Block, error) {
	var all []notionapi.Block
	var cursor notionapi.Cursor
	for {
		resp, err := f.blocks.GetChildren(ctx, blockID, &notionapi.Pagination{
			StartCursor: cursor,
			PageSize:    100,
		})
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Results...)
		if !resp.HasMore {
			return all, nil
		}
		cursor = notionapi.Cursor(resp.NextCursor)
	}
}

func pageTitle(page *notionapi.Page) string {
	if page == nil {
		return ""
	}
	for _, name := range []string{"title", "Title", "Name"} {
		if title := titlePropertyText(page.Properties[name]); title != "" {
			return title
		}
	}
	for _, prop := range page.Properties {
		if title := titlePropertyText(prop); title != "" {
			return title
		}
	}
	return page.ID.String()
}

func titlePropertyText(prop notionapi.Property) string {
	if prop == nil {
		return ""
	}
	title, ok := prop.(*notionapi.TitleProperty)
	if !ok {
		if v, ok := prop.(notionapi.TitleProperty); ok {
			title = &v
		} else {
			return ""
		}
	}
	var b strings.Builder
	for _, rt := range title.Title {
		b.WriteString(rt.PlainText)
	}
	return strings.TrimSpace(b.String())
}
