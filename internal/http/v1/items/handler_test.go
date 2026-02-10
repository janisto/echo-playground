package items

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/pagination"
	"github.com/janisto/echo-playground/internal/platform/respond"
	"github.com/janisto/echo-playground/internal/platform/validate"
)

func setupEcho() *echo.Echo {
	e := echo.New()
	e.Validator = validate.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	Register(e.Group(""))
	return e
}

func TestListItems_DefaultLimit(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var data ListData
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(data.Items) != pagination.DefaultLimit {
		t.Fatalf("expected %d items, got %d", pagination.DefaultLimit, len(data.Items))
	}
	if data.Total != len(mockItems) {
		t.Fatalf("expected total %d, got %d", len(mockItems), data.Total)
	}
}

func TestListItems_CustomLimit(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/items?limit=5", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var data ListData
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(data.Items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(data.Items))
	}
}

func TestListItems_FilterCategory(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/items?category=tools&limit=100", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var data ListData
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	for _, item := range data.Items {
		if item.Category != "tools" {
			t.Fatalf("expected category 'tools', got %q", item.Category)
		}
	}
	if data.Total == 0 {
		t.Fatal("expected at least one tool item")
	}
}

func TestListItems_InvalidCategory(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/items?category=invalid", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestListItems_InvalidCursor(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/items?cursor=!!!invalid!!!", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var problem respond.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if problem.Status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", problem.Status)
	}
}

func TestListItems_CursorTypeMismatch(t *testing.T) {
	e := setupEcho()

	cursor := pagination.Cursor{Type: "wrong", Value: "item-001"}.Encode()
	req := httptest.NewRequest(http.MethodGet, "/items?cursor="+cursor, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListItems_CursorUnknownItem(t *testing.T) {
	e := setupEcho()

	cursor := pagination.Cursor{Type: cursorType, Value: "nonexistent"}.Encode()
	req := httptest.NewRequest(http.MethodGet, "/items?cursor="+cursor, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListItems_Pagination(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/items?limit=5", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	link := rec.Header().Get("Link")
	if link == "" {
		t.Fatal("expected Link header for pagination")
	}
	if !strings.Contains(link, `rel="next"`) {
		t.Error("expected next link in Link header")
	}
}

func TestListItems_LimitTooHigh(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/items?limit=101", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestListItems_LimitZero(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/items?limit=0", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// limit=0 with omitempty should pass validation and use default limit
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var data ListData
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(data.Items) != pagination.DefaultLimit {
		t.Fatalf("expected %d items with default limit, got %d", pagination.DefaultLimit, len(data.Items))
	}
}

func TestListItems_CBOR(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/items?limit=3", nil)
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/cbor" {
		t.Fatalf("expected application/cbor, got %q", ct)
	}

	var data ListData
	if err := cbor.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to unmarshal CBOR: %v", err)
	}
	if len(data.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(data.Items))
	}
}

func TestListItems_PaginationSecondPage(t *testing.T) {
	e := setupEcho()

	// Get first page.
	req := httptest.NewRequest(http.MethodGet, "/items?limit=5", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("first page: expected 200, got %d", rec.Code)
	}

	var first ListData
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("failed to unmarshal first page: %v", err)
	}

	// Build cursor from last item on first page.
	lastID := first.Items[len(first.Items)-1].ID
	cursor := pagination.Cursor{Type: cursorType, Value: lastID}.Encode()

	req = httptest.NewRequest(http.MethodGet, "/items?limit=5&cursor="+cursor, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("second page: expected 200, got %d", rec.Code)
	}

	var second ListData
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatalf("failed to unmarshal second page: %v", err)
	}
	if len(second.Items) != 5 {
		t.Fatalf("expected 5 items on second page, got %d", len(second.Items))
	}
	if second.Items[0].ID == first.Items[0].ID {
		t.Fatal("second page should start after first page items")
	}
}

func TestListItems_EmptyCategory(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/items?category=", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Empty category with omitempty should pass validation.
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestListItems_BindError(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/items?limit=abc", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListItems_FilterCategoryWithPagination(t *testing.T) {
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/items?category=electronics&limit=2", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var data ListData
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	for _, item := range data.Items {
		if item.Category != "electronics" {
			t.Fatalf("expected category 'electronics', got %q", item.Category)
		}
	}
}
