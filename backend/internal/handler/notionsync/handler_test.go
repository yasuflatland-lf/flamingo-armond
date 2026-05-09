package notionsync

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"backend/internal/notion"
	"backend/internal/usecase"
)

type stubSyncUsecase struct {
	out   usecase.SyncFromNotionOutput
	err   error
	calls int
	in    usecase.SyncFromNotionInput
}

func (s *stubSyncUsecase) Sync(_ context.Context, in usecase.SyncFromNotionInput) (usecase.SyncFromNotionOutput, error) {
	s.calls++
	s.in = in
	if s.err != nil {
		return usecase.SyncFromNotionOutput{}, s.err
	}
	return s.out, nil
}

func TestHandlerUnauthorized(t *testing.T) {
	t.Parallel()

	uc := &stubSyncUsecase{}
	h := New(uc, Config{Token: "secret"})
	req := httptest.NewRequest(http.MethodPost, "/internal/notion-sync", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if uc.calls != 0 {
		t.Fatalf("usecase calls = %d, want 0", uc.calls)
	}
}

func TestHandlerSuccess(t *testing.T) {
	t.Parallel()

	uc := &stubSyncUsecase{out: usecase.SyncFromNotionOutput{CardgroupID: "cg-1", Inserted: 2}}
	h := New(uc, Config{
		Token:         "secret",
		PageIDs:       []string{"p1", "p2"},
		OwnerID:       "owner-1",
		CardgroupName: "English",
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/notion-sync", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if uc.calls != 1 {
		t.Fatalf("usecase calls = %d, want 1", uc.calls)
	}
	if uc.in.OwnerID != "owner-1" || uc.in.CardgroupName != "English" || len(uc.in.PageIDs) != 2 {
		t.Fatalf("input = %+v", uc.in)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["cardgroupId"] != "cg-1" {
		t.Fatalf("cardgroupId = %v, want cg-1", body["cardgroupId"])
	}
}

func TestHandlerErrorMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "fetch", err: errors.Join(usecase.ErrNotionSyncFetch, errors.New("boom")), want: http.StatusBadGateway},
		{name: "retry elapsed", err: errors.Join(notion.ErrRetryElapsed, errors.New("boom")), want: http.StatusGatewayTimeout},
		{name: "retry attempts", err: errors.Join(notion.ErrRetryAttempts, errors.New("boom")), want: http.StatusGatewayTimeout},
		{name: "context", err: context.DeadlineExceeded, want: http.StatusGatewayTimeout},
		{name: "persist", err: errors.Join(usecase.ErrNotionSyncPersist, errors.New("boom")), want: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := &stubSyncUsecase{err: tc.err}
			h := New(uc, Config{Token: "secret"})
			req := httptest.NewRequest(http.MethodPost, "/internal/notion-sync", strings.NewReader(""))
			req.Header.Set("Authorization", "Bearer secret")
			rec := httptest.NewRecorder()
			e := echo.New()
			c := e.NewContext(req, rec)

			if err := h.Handle(c); err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
