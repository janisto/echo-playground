package items

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"

	"github.com/janisto/echo-playground/internal/platform/respond"
	"github.com/janisto/echo-playground/internal/platform/timeutil"
	"github.com/janisto/echo-playground/internal/testutil"
)

func TestCanonicalCatalogMatchesEveryAcceptedValueInOrder(t *testing.T) {
	want := []string{
		"item-001|Alpha Widget|electronics|2999|USD|true|2024-01-15T10:30:00.000Z|A versatile electronic widget for everyday use",
		"item-002|Beta Gadget|electronics|4999|USD|true|2024-01-16T11:00:00.000Z|Advanced gadget with smart features",
		"item-003|Gamma Tool|tools|1550|USD|false|2024-01-17T09:15:00.000Z|Precision tool for professional work",
		"item-004|Delta Component|electronics|899|USD|true|2024-01-18T14:45:00.000Z|Essential component for electronics projects",
		"item-005|Epsilon Sensor|electronics|3499|USD|true|2024-01-19T08:00:00.000Z|High-precision environmental sensor",
		"item-006|Zeta Cable|accessories|1299|USD|true|2024-01-20T16:30:00.000Z|Premium quality data cable",
		"item-007|Eta Adapter|accessories|999|USD|false|2024-01-21T10:00:00.000Z|Universal power adapter",
		"item-008|Theta Board|electronics|8999|USD|true|2024-01-22T11:30:00.000Z|Development board for prototyping",
		"item-009|Iota Switch|electronics|599|USD|true|2024-01-23T09:45:00.000Z|Tactile push button switch",
		"item-010|Kappa Display|electronics|4599|USD|true|2024-01-24T13:00:00.000Z|OLED display module",
		"item-011|Lambda Motor|robotics|2499|USD|true|2024-01-25T08:30:00.000Z|DC motor for robotics projects",
		"item-012|Mu Servo|robotics|1899|USD|false|2024-01-26T15:00:00.000Z|High-torque servo motor",
		"item-013|Nu Battery|power|1499|USD|true|2024-01-27T10:15:00.000Z|Rechargeable lithium battery pack",
		"item-014|Xi Charger|power|2299|USD|true|2024-01-28T11:45:00.000Z|Smart battery charger",
		"item-015|Omicron Relay|electronics|799|USD|true|2024-01-29T09:00:00.000Z|5V relay module",
		"item-016|Pi Controller|electronics|5599|USD|true|2024-01-30T14:30:00.000Z|Microcontroller board",
		"item-017|Rho Resistor Kit|components|1199|USD|true|2024-02-01T08:00:00.000Z|Assorted resistor pack",
		"item-018|Sigma Capacitor Set|components|1399|USD|true|2024-02-02T10:30:00.000Z|Electrolytic capacitor assortment",
		"item-019|Tau LED Pack|components|699|USD|true|2024-02-03T11:00:00.000Z|Multi-color LED assortment",
		"item-020|Upsilon Wire Set|accessories|899|USD|false|2024-02-04T09:15:00.000Z|Jumper wire kit",
		"item-021|Phi Breadboard|tools|499|USD|true|2024-02-05T13:45:00.000Z|Solderless breadboard",
		"item-022|Chi Soldering Iron|tools|3599|USD|true|2024-02-06T10:00:00.000Z|Temperature-controlled soldering station",
		"item-023|Psi Multimeter|tools|4299|USD|true|2024-02-07T11:30:00.000Z|Digital multimeter with auto-ranging",
		"item-024|Omega Oscilloscope|tools|29999|USD|true|2024-02-08T14:00:00.000Z|Portable digital oscilloscope",
		"item-025|Alpha Pro Widget|electronics|5999|USD|true|2024-02-09T08:30:00.000Z|Professional-grade widget with extended features",
		"item-026|Beta Max Gadget|electronics|7999|USD|false|2024-02-10T09:00:00.000Z|Maximum performance gadget",
		"item-027|Gamma Plus Tool|tools|2599|USD|true|2024-02-11T10:15:00.000Z|Enhanced precision tool",
		"item-028|Delta Ultra Component|electronics|1699|USD|true|2024-02-12T11:45:00.000Z|Ultra-reliable component",
		"item-029|Epsilon HD Sensor|electronics|5499|USD|true|2024-02-13T13:00:00.000Z|High-definition sensor array",
		"item-030|Zeta Premium Cable|accessories|1999|USD|true|2024-02-14T15:30:00.000Z|Gold-plated premium cable",
	}
	if len(catalog) != len(want) {
		t.Fatalf("catalog length = %d, want %d", len(catalog), len(want))
	}
	for index, item := range catalog {
		got := fmt.Sprintf(
			"%s|%s|%s|%d|%s|%t|%s|%s",
			item.ID,
			item.Name,
			item.Category,
			item.Price.AmountMinor,
			item.Price.Currency,
			item.InStock,
			item.CreatedAt.UTC().Format(timeutil.RFC3339Millis),
			item.Description,
		)
		if got != want[index] {
			t.Fatalf("catalog item %d = %q, want %q", index+1, got, want[index])
		}
	}
}

func TestCatalogDefaultPageAndUSDModel(t *testing.T) {
	rec := serveItems(t, "/v1/items", "application/json")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status/media = %d/%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var page ListData
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 30 || len(page.Items) != 20 || page.Items[0].ID != "item-001" || page.Items[19].ID != "item-020" {
		t.Fatalf("default page = %#v", page)
	}
	for _, item := range page.Items {
		if item.Price.Currency != "USD" || item.Price.AmountMinor < 0 {
			t.Fatalf("noncanonical money: %#v", item.Price)
		}
	}
	if got := rec.Header().
		Get("Link"); !strings.Contains(got, "</v1/items?") || !strings.Contains(got, "limit=20") ||
		!strings.Contains(got, `rel="next"`) {
		t.Fatalf("Link = %q", got)
	}
}

func TestCategoryFilteringPrecedesPagination(t *testing.T) {
	wants := map[string]int{"electronics": 13, "tools": 6, "accessories": 4, "robotics": 2, "power": 2, "components": 3}
	for category, total := range wants {
		rec := serveItems(t, "/v1/items?category="+category+"&limit=100", "")
		var page ListData
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &page) != nil {
			t.Fatalf("%s response = %d %s", category, rec.Code, rec.Body.String())
		}
		if page.Total != total || len(page.Items) != total {
			t.Fatalf("%s count = %d/%d, want %d", category, len(page.Items), page.Total, total)
		}
		for _, item := range page.Items {
			if item.Category != category {
				t.Fatalf("%s page contains %#v", category, item)
			}
		}
	}
}

func TestPaginationTraversesThreePagesAndBack(t *testing.T) {
	target := "/v1/items?limit=10"
	seen := make([]string, 0, 30)
	links := make([]string, 0, 3)
	for pageIndex := range 3 {
		rec := serveItems(t, target, "")
		var page ListData
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &page) != nil {
			t.Fatalf("page %d = %d %s", pageIndex, rec.Code, rec.Body.String())
		}
		for _, item := range page.Items {
			seen = append(seen, item.ID)
		}
		links = append(links, rec.Header().Get("Link"))
		if pageIndex < 2 {
			target = linkTarget(t, links[pageIndex], "next")
		}
	}
	if len(seen) != 30 {
		t.Fatalf("saw %d items", len(seen))
	}
	for index, id := range seen {
		want := "item-" + threeDigit(index+1)
		if id != want {
			t.Fatalf("item %d = %s, want %s", index, id, want)
		}
	}
	if strings.Contains(links[0], `rel="prev"`) || strings.Contains(links[2], `rel="next"`) {
		t.Fatalf("boundary links = %#v", links)
	}
	backTarget := linkTarget(t, links[2], "prev")
	back := serveItems(t, backTarget, "")
	var page ListData
	if err := json.Unmarshal(back.Body.Bytes(), &page); err != nil || page.Items[0].ID != "item-011" {
		t.Fatalf("back page = %#v, err=%v", page, err)
	}
}

func TestItemsJSONAndCBORCarryEquivalentMembers(t *testing.T) {
	jsonResponse := serveItems(t, "/v1/items?limit=1", "application/json")
	cborResponse := serveItems(t, "/v1/items?limit=1", "application/cbor")
	var jsonObject, cborObject map[string]any
	if err := json.Unmarshal(jsonResponse.Body.Bytes(), &jsonObject); err != nil {
		t.Fatal(err)
	}
	if err := cbor.Unmarshal(cborResponse.Body.Bytes(), &cborObject); err != nil {
		t.Fatal(err)
	}
	jsonTotal, jsonOK := jsonObject["total"].(float64)
	cborTotal, cborOK := cborObject["total"].(uint64)
	if len(jsonObject) != 2 || len(cborObject) != 2 || !jsonOK || !cborOK ||
		jsonTotal != float64(cborTotal) {
		t.Fatalf("JSON/CBOR = %#v / %#v", jsonObject, cborObject)
	}
}

func TestItemsStrictQueryAndCursorFailures(t *testing.T) {
	validCursor := nextCursor(t, "/v1/items?limit=1")
	tests := []struct {
		query  string
		status int
		code   string
	}{
		{"unknown=1", 400, "invalid_request"},
		{"limit=1&limit=2", 400, "invalid_request"},
		{"limit=%FF", 400, "invalid_request"},
		{"cursor=", 400, "invalid_request"},
		{"cursor=malformed", 400, "invalid_request"},
		{"cursor=" + strings.Repeat("a", 2049), 400, "invalid_request"},
		{"limit=0", 422, "validation_failed"},
		{"limit=101", 422, "validation_failed"},
		{"limit=1.0", 422, "validation_failed"},
		{"category=unknown", 422, "validation_failed"},
		{"limit=2&cursor=" + url.QueryEscape(validCursor), 400, "invalid_request"},
	}
	for _, test := range tests {
		rec := serveItems(t, "/v1/items?"+test.query, "")
		assertItemProblem(t, rec, test.status, test.code)
		if strings.Contains(rec.Body.String(), "unknown") ||
			strings.Contains(rec.Body.String(), strings.Repeat("a", 50)) {
			t.Fatalf("error reflected rejected input: %s", rec.Body.String())
		}
	}
}

func TestItemsUnacceptableAndUnsupportedMethod(t *testing.T) {
	rec := serveItems(t, "/v1/items", "text/plain")
	assertItemProblem(t, rec, 406, "not_acceptable")
	e := itemsEcho()
	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/items",
		strings.NewReader(strings.Repeat("x", 100)),
	)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assertItemProblem(t, rec, 405, "method_not_allowed")
	if !strings.Contains(rec.Header().Get("Allow"), "GET") {
		t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
	}
}

func serveItems(t *testing.T, target, accept string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	req.Host = "attacker.invalid"
	rec := httptest.NewRecorder()
	itemsEcho().ServeHTTP(rec, req)
	return rec
}

func itemsEcho() http.Handler {
	e := testutil.NewTestEcho()
	Register(e.Group("/v1"))
	return e
}

func nextCursor(t *testing.T, target string) string {
	t.Helper()
	return queryFromLink(t, serveItems(t, target, "").Header().Get("Link"), "next").Get("cursor")
}

func linkTarget(t *testing.T, link, relation string) string {
	t.Helper()
	for part := range strings.SplitSeq(link, ",") {
		if strings.Contains(part, `rel="`+relation+`"`) {
			start, end := strings.Index(part, "<"), strings.Index(part, ">")
			if start >= 0 && end > start {
				return part[start+1 : end]
			}
		}
	}
	t.Fatalf("Link %q lacks %s", link, relation)
	return ""
}

func queryFromLink(t *testing.T, link, relation string) url.Values {
	t.Helper()
	target, err := url.Parse(linkTarget(t, link, relation))
	if err != nil {
		t.Fatal(err)
	}
	return target.Query()
}

func assertItemProblem(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var problem respond.ProblemDetails
	if recorder.Code != status || json.Unmarshal(recorder.Body.Bytes(), &problem) != nil || problem.Code != code {
		t.Fatalf("response = %d %#v: %s", recorder.Code, problem, recorder.Body.String())
	}
}

func threeDigit(value int) string {
	return fmt.Sprintf("%03d", value)
}
