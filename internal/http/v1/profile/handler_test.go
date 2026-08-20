package profile

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"

	"github.com/janisto/echo-playground/internal/platform/auth"
	"github.com/janisto/echo-playground/internal/platform/request"
	"github.com/janisto/echo-playground/internal/platform/respond"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
	"github.com/janisto/echo-playground/internal/testutil"
	testfake "github.com/janisto/echo-playground/internal/testutil/fake"
)

const validCreate = `{"firstName":"Ada","lastName":"Lovelace","contactEmail":" Ada@EXAMPLE.com ","phoneNumber":" +358401234567 ","termsAccepted":true}`

type verifierDouble struct {
	calls atomic.Int32
	user  *auth.FirebaseUser
	err   error
}

func (verifier *verifierDouble) Verify(context.Context, string) (*auth.FirebaseUser, error) {
	verifier.calls.Add(1)
	return verifier.user, verifier.err
}

func TestProfileLifecycleNormalizationNoOpAndDelete(t *testing.T) {
	store := testfake.NewProfileStore()
	verifier := &verifierDouble{user: &auth.FirebaseUser{UID: "principal-123"}}
	handler := profileEcho(verifier, store)

	create := profileRequest(t, handler, http.MethodPost, validCreate, "application/json", "application/json")
	if create.Code != 201 || create.Header().Get("Location") != "/v1/profile" {
		t.Fatalf("create = %d Location=%q %s", create.Code, create.Header().Get("Location"), create.Body.String())
	}
	var created Profile
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != "principal-123" || created.ContactEmail != "Ada@example.com" ||
		created.PhoneNumber != "+358401234567" ||
		created.MarketingOptIn ||
		!created.TermsAccepted ||
		!created.CreatedAt.Equal(created.UpdatedAt.Time) {
		t.Fatalf("created profile = %#v", created)
	}
	if store.WriteCount() != 1 {
		t.Fatalf("create writes = %d", store.WriteCount())
	}

	get := profileRequest(t, handler, http.MethodGet, "", "", "application/json")
	if get.Code != 200 || get.Body.String() != create.Body.String() {
		t.Fatalf("get = %d %s, create=%s", get.Code, get.Body.String(), create.Body.String())
	}

	noOp := profileRequest(
		t,
		handler,
		http.MethodPatch,
		`{"contactEmail":"Ada@EXAMPLE.com"}`,
		"application/json",
		"application/json",
	)
	var unchanged Profile
	if noOp.Code != 200 || json.Unmarshal(noOp.Body.Bytes(), &unchanged) != nil ||
		!unchanged.UpdatedAt.Equal(created.UpdatedAt.Time) ||
		store.WriteCount() != 1 {
		t.Fatalf("no-op = %d %#v writes=%d", noOp.Code, unchanged, store.WriteCount())
	}

	changed := profileRequest(
		t,
		handler,
		http.MethodPatch,
		`{"marketingOptIn":true}`,
		"application/json",
		"application/json",
	)
	var updated Profile
	if changed.Code != 200 || json.Unmarshal(changed.Body.Bytes(), &updated) != nil || !updated.MarketingOptIn ||
		!updated.UpdatedAt.After(updated.CreatedAt.Time) || store.WriteCount() != 2 {
		t.Fatalf("changed = %d %#v writes=%d", changed.Code, updated, store.WriteCount())
	}

	deleted := profileRequest(t, handler, http.MethodDelete, "", "", "text/plain")
	if deleted.Code != 204 || deleted.Body.Len() != 0 || deleted.Header().Get("Content-Type") != "" ||
		deleted.Header().Get("Content-Length") != "" {
		t.Fatalf(
			"delete = %d media=%q length=%q body=%q",
			deleted.Code,
			deleted.Header().Get("Content-Type"),
			deleted.Header().Get("Content-Length"),
			deleted.Body.String(),
		)
	}
	missing := profileRequest(t, handler, http.MethodGet, "", "", "")
	assertProfileProblem(t, missing, 404, "profile_not_found")
}

func TestProfileCBORRequestAndResponse(t *testing.T) {
	store := testfake.NewProfileStore()
	verifier := &verifierDouble{user: &auth.FirebaseUser{UID: "principal-cbor"}}
	body, err := cbor.Marshal(map[string]any{
		"firstName": "Ada", "lastName": "Lovelace", "contactEmail": "Ada@EXAMPLE.com",
		"phoneNumber": "+358401234567", "marketingOptIn": false, "termsAccepted": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/profile", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/cbor")
	req.Header.Set("Accept", "application/cbor")
	rec := httptest.NewRecorder()
	profileEcho(verifier, store).ServeHTTP(rec, req)
	if rec.Code != 201 || rec.Header().Get("Content-Type") != "application/cbor" {
		t.Fatalf("CBOR create = %d/%q %x", rec.Code, rec.Header().Get("Content-Type"), rec.Body.Bytes())
	}
	var object map[string]any
	if err := cbor.Unmarshal(
		rec.Body.Bytes(),
		&object,
	); err != nil || len(object) != 9 ||
		object["termsAccepted"] != true {
		t.Fatalf("CBOR profile = %#v, err=%v", object, err)
	}
}

func TestAuthenticationAndNegotiationPrecedeBodyAndPersistence(t *testing.T) {
	store := testfake.NewProfileStore()
	verifier := &verifierDouble{user: &auth.FirebaseUser{UID: "principal"}}
	handler := profileEcho(verifier, store)

	missing := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/profile", strings.NewReader(`{`))
	missing.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, missing)
	assertProfileProblem(t, rec, 401, "unauthorized")
	if verifier.calls.Load() != 0 || store.WriteCount() != 0 {
		t.Fatalf("missing credential calls=%d writes=%d", verifier.calls.Load(), store.WriteCount())
	}

	badVerifier := &verifierDouble{err: auth.ErrInvalidToken}
	rec = profileRequest(t, profileEcho(badVerifier, store), http.MethodPost, `{`, "application/json", "")
	assertProfileProblem(t, rec, 401, "unauthorized")
	if badVerifier.calls.Load() != 1 || store.WriteCount() != 0 {
		t.Fatalf("invalid credential calls=%d writes=%d", badVerifier.calls.Load(), store.WriteCount())
	}

	before := verifier.calls.Load()
	rec = profileRequest(t, handler, http.MethodPost, validCreate, "application/json", "text/plain")
	assertProfileProblem(t, rec, 406, "not_acceptable")
	if verifier.calls.Load() != before || store.WriteCount() != 0 {
		t.Fatalf("negotiation invoked auth or storage: calls=%d writes=%d", verifier.calls.Load(), store.WriteCount())
	}
}

func TestProfileAuthenticationFailuresAreIndistinguishableAtHTTPBoundary(t *testing.T) {
	tests := []struct {
		name          string
		authorization []string
		verifierError error
		verifierCalls int32
		status        int
	}{
		{name: "missing", status: 401},
		{name: "other scheme", authorization: []string{"Basic abc"}, status: 401},
		{name: "tab separator", authorization: []string{"Bearer\ttoken"}, status: 401},
		{name: "comma combined", authorization: []string{"Bearer first,Bearer second"}, status: 401},
		{name: "duplicate fields", authorization: []string{"Bearer first", "Bearer second"}, status: 401},
		{
			name:          "expired",
			authorization: []string{"Bearer token"},
			verifierError: auth.ErrTokenExpired,
			verifierCalls: 1,
			status:        401,
		},
		{
			name:          "revoked",
			authorization: []string{"Bearer token"},
			verifierError: auth.ErrTokenRevoked,
			verifierCalls: 1,
			status:        401,
		},
		{
			name:          "disabled user",
			authorization: []string{"Bearer token"},
			verifierError: auth.ErrUserDisabled,
			verifierCalls: 1,
			status:        401,
		},
		{
			name:          "wrong audience",
			authorization: []string{"Bearer token"},
			verifierError: auth.ErrInvalidToken,
			verifierCalls: 1,
			status:        401,
		},
		{
			name:          "wrong issuer",
			authorization: []string{"Bearer token"},
			verifierError: auth.ErrInvalidToken,
			verifierCalls: 1,
			status:        401,
		},
		{
			name:          "unknown key",
			authorization: []string{"Bearer token"},
			verifierError: auth.ErrInvalidToken,
			verifierCalls: 1,
			status:        401,
		},
		{
			name:          "corrupt signature",
			authorization: []string{"Bearer token"},
			verifierError: auth.ErrInvalidToken,
			verifierCalls: 1,
			status:        401,
		},
		{
			name:          "none algorithm",
			authorization: []string{"Bearer token"},
			verifierError: auth.ErrInvalidToken,
			verifierCalls: 1,
			status:        401,
		},
		{
			name:          "symmetric algorithm",
			authorization: []string{"Bearer token"},
			verifierError: auth.ErrInvalidToken,
			verifierCalls: 1,
			status:        401,
		},
		{
			name:          "verifier unavailable",
			authorization: []string{"Bearer token"},
			verifierError: auth.ErrAuthUnavailable,
			verifierCalls: 1,
			status:        503,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifier := &verifierDouble{err: test.verifierError}
			e := profileEcho(verifier, noCallProfileService{})
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/profile", nil)
			for _, value := range test.authorization {
				req.Header.Add("Authorization", value)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assertProfileProblem(
				t,
				rec,
				test.status,
				map[bool]string{true: "dependency_unavailable", false: "unauthorized"}[test.status == 503],
			)
			if verifier.calls.Load() != test.verifierCalls {
				t.Fatalf("verifier calls = %d, want %d", verifier.calls.Load(), test.verifierCalls)
			}
			if test.status == 401 && rec.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatalf("challenge = %q", rec.Header().Get("WWW-Authenticate"))
			}
			if strings.Contains(rec.Body.String(), test.name) {
				t.Fatalf("authentication reason leaked: %s", rec.Body.String())
			}
		})
	}
}

func TestProfileStrictValidationHasNoPersistenceSideEffect(t *testing.T) {
	tests := []struct {
		method string
		body   string
		status int
		code   string
	}{
		{http.MethodPost, `{"firstName":"Ada","firstName":"Grace"}`, 400, "invalid_request"},
		{
			http.MethodPost,
			`{"firstName":"Ada","lastName":"Lovelace","contactEmail":"ada@example.com","phoneNumber":"+358401234567","termsAccepted":false}`,
			422,
			"validation_failed",
		},
		{
			http.MethodPost,
			`{"firstName":" Ada","lastName":"Lovelace","contactEmail":"ada@example.com","phoneNumber":"+358401234567","termsAccepted":true}`,
			422,
			"validation_failed",
		},
		{
			http.MethodPost,
			`{"firstName":"Ada","lastName":"Lovelace","contactEmail":"ada@example.com","phoneNumber":"+358401234567","termsAccepted":true,"unknown":"secret"}`,
			422,
			"validation_failed",
		},
		{
			http.MethodPost,
			`{"firstName":"Ada","lastName":"Lovelace","contactEmail":"Ada@example.\u212AOM","phoneNumber":"+358401234567","termsAccepted":true}`,
			422,
			"validation_failed",
		},
		{
			http.MethodPost,
			`{"firstName":"Ada","lastName":"Lovelace","contactEmail":"ada@example.com","phoneNumber":"+358401234567","marketingOptIn":null,"termsAccepted":true}`,
			422,
			"validation_failed",
		},
		{http.MethodPatch, `{}`, 422, "validation_failed"},
		{http.MethodPatch, `{"marketingOptIn":null,"firstName":"Ada"}`, 422, "validation_failed"},
		{http.MethodPatch, `{"termsAccepted":true}`, 422, "validation_failed"},
		{http.MethodPatch, `{"contactEmail":"not-an-address"}`, 422, "validation_failed"},
	}
	for _, test := range tests {
		store := testfake.NewProfileStore()
		verifier := &verifierDouble{user: &auth.FirebaseUser{UID: "principal"}}
		rec := profileRequest(t, profileEcho(verifier, store), test.method, test.body, "application/json", "")
		assertProfileProblem(t, rec, test.status, test.code)
		if store.WriteCount() != 0 || strings.Contains(rec.Body.String(), "secret") ||
			strings.Contains(rec.Body.String(), "not-an-address") {
			t.Fatalf("rejection leaked or wrote: writes=%d body=%s", store.WriteCount(), rec.Body.String())
		}
	}
}

func TestProfileBodyLimitDeclaredAndStreamed(t *testing.T) {
	for _, size := range []int{999_999, 1_000_000, 1_000_001} {
		body := paddedProfileBody(t, size)
		store := testfake.NewProfileStore()
		verifier := &verifierDouble{user: &auth.FirebaseUser{UID: "principal"}}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/profile", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		profileEcho(verifier, store).ServeHTTP(rec, req)
		if int64(size) <= request.BodyLimit {
			assertProfileProblem(t, rec, 422, "validation_failed")
			if verifier.calls.Load() != 1 {
				t.Fatalf("size %d auth calls = %d", size, verifier.calls.Load())
			}
		} else {
			assertProfileProblem(t, rec, 413, "payload_too_large")
			if verifier.calls.Load() != 0 {
				t.Fatalf("declared over-limit invoked auth")
			}
		}
		if store.WriteCount() != 0 {
			t.Fatalf("size %d wrote profile", size)
		}
	}

	store := testfake.NewProfileStore()
	verifier := &verifierDouble{user: &auth.FirebaseUser{UID: "principal"}}
	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/profile",
		strings.NewReader(paddedProfileBody(t, 1_000_001)),
	)
	req.ContentLength = -1
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	profileEcho(verifier, store).ServeHTTP(rec, req)
	assertProfileProblem(t, rec, 413, "payload_too_large")
	if verifier.calls.Load() != 1 || store.WriteCount() != 0 {
		t.Fatalf("streamed over-limit auth=%d writes=%d", verifier.calls.Load(), store.WriteCount())
	}
}

func TestConcurrentProfileCreateAndDeleteOutcomes(t *testing.T) {
	store := testfake.NewProfileStore()
	handler := profileEcho(&verifierDouble{user: &auth.FirebaseUser{UID: "principal"}}, store)
	statuses := concurrentRequests(t, handler, http.MethodPost, validCreate, "application/json")
	assertStatusPair(t, statuses, 201, 409)
	if store.WriteCount() != 1 {
		t.Fatalf("concurrent create writes = %d", store.WriteCount())
	}
	statuses = concurrentRequests(t, handler, http.MethodDelete, "", "")
	assertStatusPair(t, statuses, 204, 404)
	if store.WriteCount() != 2 {
		t.Fatalf("concurrent delete writes = %d", store.WriteCount())
	}
}

func TestConcurrentProfilePatchesAreAtomicAndDeleteDoesNotResurrect(t *testing.T) {
	store := testfake.NewProfileStore()
	handler := profileEcho(&verifierDouble{user: &auth.FirebaseUser{UID: "principal"}}, store)
	if created := profileRequest(
		t,
		handler,
		http.MethodPost,
		validCreate,
		"application/json",
		"",
	); created.Code != 201 {
		t.Fatalf("create = %d: %s", created.Code, created.Body.String())
	}

	start := make(chan struct{})
	statuses := make(chan int, 2)
	var wait sync.WaitGroup
	for _, body := range []string{`{"firstName":"Grace"}`, `{"lastName":"Hopper"}`} {
		wait.Go(func() {
			<-start
			statuses <- profileRequest(t, handler, http.MethodPatch, body, "application/json", "").Code
		})
	}
	close(start)
	wait.Wait()
	close(statuses)
	for status := range statuses {
		if status != 200 {
			t.Fatalf("concurrent patch status = %d", status)
		}
	}
	stored, err := store.Get(t.Context(), "principal")
	if err != nil || stored.FirstName != "Grace" || stored.LastName != "Hopper" || store.WriteCount() != 3 {
		t.Fatalf("atomic patch state = %#v, err=%v writes=%d", stored, err, store.WriteCount())
	}

	start = make(chan struct{})
	statuses = make(chan int, 2)
	wait = sync.WaitGroup{}
	for _, request := range []struct {
		method string
		body   string
		media  string
	}{{http.MethodPatch, `{"marketingOptIn":true}`, "application/json"}, {http.MethodDelete, "", ""}} {
		wait.Go(func() {
			<-start
			statuses <- profileRequest(t, handler, request.method, request.body, request.media, "").Code
		})
	}
	close(start)
	wait.Wait()
	close(statuses)
	gotStatuses := make([]int, 0, 2)
	for status := range statuses {
		gotStatuses = append(gotStatuses, status)
	}
	deleteCount, patchOutcomeOK := 0, false
	for _, status := range gotStatuses {
		if status == 204 {
			deleteCount++
		}
		if status == 200 || status == 404 {
			patchOutcomeOK = true
		}
	}
	if deleteCount != 1 || !patchOutcomeOK {
		t.Fatalf("patch/delete statuses = %v", gotStatuses)
	}
	if _, err := store.Get(t.Context(), "principal"); !errors.Is(err, profilesvc.ErrNotFound) {
		t.Fatalf("patch/delete resurrected profile: %v", err)
	}
}

func TestProfileDependencyFailureMapping(t *testing.T) {
	verifier := &verifierDouble{err: auth.ErrAuthUnavailable}
	store := testfake.NewProfileStore()
	rec := profileRequest(t, profileEcho(verifier, store), http.MethodGet, "", "", "")
	assertProfileProblem(t, rec, 503, "dependency_unavailable")
	if rec.Header().Get("Retry-After") != "" || store.WriteCount() != 0 {
		t.Fatalf(
			"generic dependency response has retry/write: %q/%d",
			rec.Header().Get("Retry-After"),
			store.WriteCount(),
		)
	}

	service := &failingService{err: errors.New("provider secret")}
	rec = profileRequest(
		t,
		profileEcho(&verifierDouble{user: &auth.FirebaseUser{UID: "principal"}}, service),
		http.MethodGet,
		"",
		"",
		"",
	)
	assertProfileProblem(t, rec, 500, "internal_error")
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("service error leaked: %s", rec.Body.String())
	}
}

func TestProfileDefinitiveOutcomeWinsExpiredRequestDeadline(t *testing.T) {
	deadlineContext, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()

	for _, test := range []struct {
		method string
		body   string
		error  error
		status int
		code   string
	}{
		{method: http.MethodPost, body: validCreate, error: profilesvc.ErrAlreadyExists, status: 409, code: "profile_exists"},
		{method: http.MethodGet, error: profilesvc.ErrNotFound, status: 404, code: "profile_not_found"},
	} {
		req := httptest.NewRequestWithContext(
			deadlineContext,
			test.method,
			"/v1/profile",
			strings.NewReader(test.body),
		)
		req.Header.Set("Authorization", "Bearer token")
		if test.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		profileEcho(
			&verifierDouble{user: &auth.FirebaseUser{UID: "principal"}},
			&failingService{err: test.error},
		).ServeHTTP(rec, req)
		assertProfileProblem(t, rec, test.status, test.code)
	}
}

type failingService struct{ err error }

type noCallProfileService struct{}

func (noCallProfileService) Create(context.Context, string, profilesvc.CreateParams) (*profilesvc.Profile, error) {
	panic("authentication failure reached persistence")
}

func (noCallProfileService) Get(context.Context, string) (*profilesvc.Profile, error) {
	panic("authentication failure reached persistence")
}

func (noCallProfileService) Update(context.Context, string, profilesvc.UpdateParams) (*profilesvc.Profile, error) {
	panic("authentication failure reached persistence")
}

func (noCallProfileService) Delete(context.Context, string) error {
	panic("authentication failure reached persistence")
}

func (service *failingService) Create(context.Context, string, profilesvc.CreateParams) (*profilesvc.Profile, error) {
	return nil, service.err
}

func (service *failingService) Get(context.Context, string) (*profilesvc.Profile, error) {
	return nil, service.err
}

func (service *failingService) Update(context.Context, string, profilesvc.UpdateParams) (*profilesvc.Profile, error) {
	return nil, service.err
}
func (service *failingService) Delete(context.Context, string) error { return service.err }

func profileEcho(verifier auth.Verifier, service profilesvc.Service) http.Handler {
	e := testutil.NewTestEcho()
	e.Use(request.BodyLimitMiddleware())
	group := e.Group("/v1", respond.SuccessNegotiation(false), auth.Middleware(verifier))
	Register(group, service)
	return e
}

func profileRequest(
	t *testing.T,
	handler http.Handler,
	method, body, contentType, accept string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, "/v1/profile", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func assertProfileProblem(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var problem respond.ProblemDetails
	if recorder.Code != status || json.Unmarshal(recorder.Body.Bytes(), &problem) != nil || problem.Code != code {
		t.Fatalf("response = %d %#v: %s", recorder.Code, problem, recorder.Body.String())
	}
}

func paddedProfileBody(t *testing.T, size int) string {
	t.Helper()
	base := strings.TrimSuffix(validCreate, "}") + `,"padding":""}`
	padding := size - len(base)
	if padding < 0 {
		t.Fatal("profile fixture too large")
	}
	body := strings.Replace(base, `"padding":""`, `"padding":"`+strings.Repeat("x", padding)+`"`, 1)
	if len(body) != size {
		t.Fatalf("body size = %d, want %d", len(body), size)
	}
	return body
}

func concurrentRequests(t *testing.T, handler http.Handler, method, body, contentType string) []int {
	t.Helper()
	start := make(chan struct{})
	statuses := make(chan int, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Go(func() {
			<-start
			statuses <- profileRequest(t, handler, method, body, contentType, "").Code
		})
	}
	close(start)
	wait.Wait()
	close(statuses)
	result := make([]int, 0, 2)
	for status := range statuses {
		result = append(result, status)
	}
	return result
}

func assertStatusPair(t *testing.T, actual []int, first, second int) {
	t.Helper()
	forward := len(actual) == 2 && actual[0] == first && actual[1] == second
	reverse := len(actual) == 2 && actual[0] == second && actual[1] == first
	if !forward && !reverse {
		t.Fatalf("statuses = %v, want %d and %d", actual, first, second)
	}
}

var (
	_ auth.Verifier      = (*verifierDouble)(nil)
	_ profilesvc.Service = (*failingService)(nil)
	_ profilesvc.Service = noCallProfileService{}
)
