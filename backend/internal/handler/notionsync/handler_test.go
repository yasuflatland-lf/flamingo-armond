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
	"github.com/rotisserie/eris"

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

func TestHandlerUnauthorizedVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		auth string // value of Authorization header; "" omits the header
	}{
		{name: "wrong token", auth: "Bearer wrong"},
		{name: "malformed header (no Bearer prefix)", auth: "foo bar"},
		{name: "lowercase scheme", auth: "bearer secret"},
		{name: "empty token after Bearer", auth: "Bearer "},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := &stubSyncUsecase{}
			h := New(uc, Config{Token: "secret"})
			req := httptest.NewRequest(http.MethodPost, "/internal/notion-sync", nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
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
				t.Fatalf("usecase calls = %d, want 0 (auth must reject before invoking usecase)", uc.calls)
			}
		})
	}
}

func TestHandlerSuccess(t *testing.T) {
	t.Parallel()

	uc := &stubSyncUsecase{out: usecase.SyncFromNotionOutput{
		CardgroupID: "cg-1",
		Inserted:    2,
		Updated:     3,
		Deleted:     1,
	}}
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
	// JSON numbers decode as float64; assert each count survives the wire
	// contract — these keys are consumed by the scheduler runbook so a
	// rename would break external tooling silently.
	if body["inserted"] != float64(2) {
		t.Fatalf("inserted = %v, want 2", body["inserted"])
	}
	if body["updated"] != float64(3) {
		t.Fatalf("updated = %v, want 3", body["updated"])
	}
	if body["deleted"] != float64(1) {
		t.Fatalf("deleted = %v, want 1", body["deleted"])
	}
}

func TestHandlerSuccess_ParseErrorsJSONShape(t *testing.T) {
	t.Parallel()

	uc := &stubSyncUsecase{out: usecase.SyncFromNotionOutput{
		CardgroupID: "cg-1",
		ParseErrors: []usecase.DictionaryValidationError{
			{Line: 2, Message: "duplicate front in Notion pages (later occurrence wins)", Front: "apple", Back: "fruit"},
			{Line: 3, Message: "syntax error: unexpected ..."},
		},
	}}
	h := New(uc, Config{Token: "secret"})
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

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	rawErrs, ok := body["parseErrors"]
	if !ok {
		t.Fatal("parseErrors key missing from response body")
	}

	var parseErrors []map[string]any
	if err := json.Unmarshal(rawErrs, &parseErrors); err != nil {
		t.Fatalf("decode parseErrors: %v", err)
	}
	if len(parseErrors) != 2 {
		t.Fatalf("parseErrors len = %d, want 2", len(parseErrors))
	}

	// Entry 0: all four fields must be present with correct values.
	e0 := parseErrors[0]
	if e0["line"] != float64(2) {
		t.Fatalf("parseErrors[0].line = %v, want 2", e0["line"])
	}
	if e0["message"] != "duplicate front in Notion pages (later occurrence wins)" {
		t.Fatalf("parseErrors[0].message = %v", e0["message"])
	}
	if e0["front"] != "apple" {
		t.Fatalf("parseErrors[0].front = %v, want apple", e0["front"])
	}
	if e0["back"] != "fruit" {
		t.Fatalf("parseErrors[0].back = %v, want fruit", e0["back"])
	}

	// Entry 1: front and back must be absent (omitempty drops zero-value strings).
	e1 := parseErrors[1]
	if e1["line"] != float64(3) {
		t.Fatalf("parseErrors[1].line = %v, want 3", e1["line"])
	}
	if _, ok := e1["front"]; ok {
		t.Fatal("parseErrors[1] must not have front key (omitempty), but it is present")
	}
	if _, ok := e1["back"]; ok {
		t.Fatal("parseErrors[1] must not have back key (omitempty), but it is present")
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
		{name: "invalid input", err: errors.Join(usecase.ErrNotionSyncInvalidInput, errors.New("boom")), want: http.StatusUnprocessableEntity},
		{name: "parse", err: errors.Join(usecase.ErrNotionSyncParse, errors.New("boom")), want: http.StatusUnprocessableEntity},
		{name: "cap exceeded", err: eris.Wrap(usecase.ErrNotionSyncInvalidInput, "parsed rows exceed cap"), want: http.StatusUnprocessableEntity},
		{name: "deps not configured", err: eris.Wrap(usecase.ErrNotionSyncInvalidInput, "dependencies are not configured"), want: http.StatusUnprocessableEntity},
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
