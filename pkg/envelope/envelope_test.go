package envelope

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/combi/pkg/apperror"
	pkgerrors "github.com/combi/pkg/errors"
	"github.com/combi/pkg/middleware"
)

func TestSuccessEnvelopeShape(t *testing.T) {
	ctx := middleware.WithRequestID(
		middleware.WithTenantID(context.Background(), "ten_1"), "req_env",
	)

	rec := httptest.NewRecorder()
	err := New(ctx, map[string]string{"id": "idn_1"}).
		WithEvent("identity.created", "idn_1").
		WithLink("self", "/identities/idn_1").
		WithLink("verify", "/identities/idn_1/verify").
		Write(rec)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// All five envelope keys must always be present.
	for _, key := range []string{"data", "meta", "error", "events"} {
		if _, ok := body[key]; !ok {
			t.Errorf("envelope missing %q key", key)
		}
	}

	var meta Meta
	if err := json.Unmarshal(body["meta"], &meta); err != nil {
		t.Fatalf("meta: %v", err)
	}
	if meta.RequestID != "req_env" {
		t.Errorf("meta.request_id = %q, want req_env", meta.RequestID)
	}
	if meta.TenantID != "ten_1" {
		t.Errorf("meta.tenant_id = %q, want ten_1", meta.TenantID)
	}
	if meta.Version != APIVersion {
		t.Errorf("meta.version = %q, want %q", meta.Version, APIVersion)
	}

	var events []EventRef
	if err := json.Unmarshal(body["events"], &events); err != nil {
		t.Fatalf("events: %v", err)
	}
	if len(events) != 1 || events[0].Type != "identity.created" {
		t.Errorf("events = %v", events)
	}

	var links map[string]string
	if err := json.Unmarshal(body["_links"], &links); err != nil {
		t.Fatalf("_links: %v", err)
	}
	if links["self"] != "/identities/idn_1" {
		t.Errorf("_links.self = %q", links["self"])
	}

	// error must be JSON null on success
	if string(body["error"]) != "null" {
		t.Errorf("error = %s, want null", body["error"])
	}
}

func TestErrorEnvelopeFromAfros(t *testing.T) {
	ctx := middleware.WithRequestID(context.Background(), "req_err")
	afros := pkgerrors.NotFound("identity", "idn_x")

	rec := httptest.NewRecorder()
	NewError(ctx, afros).Write(rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}

	var body struct {
		Data  any                  `json:"data"`
		Meta  Meta                 `json:"meta"`
		Error *pkgerrors.HTTPError `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Error == nil || body.Error.Code != "NOT_FOUND" {
		t.Errorf("error = %+v, want NOT_FOUND", body.Error)
	}
	if body.Data != nil {
		t.Errorf("data must be null on error, got %v", body.Data)
	}
	if body.Meta.RequestID != "req_err" {
		t.Errorf("meta.request_id = %q", body.Meta.RequestID)
	}
}

func TestErrorEnvelopeFromPlainErrorDoesNotLeak(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil).WithContext(
		middleware.WithRequestID(context.Background(), "req_leak"),
	)

	WriteError(rec, req, errors.New("postgres: password=hunter2"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if got := rec.Body.String(); strings.Contains(got, "hunter2") {
		t.Errorf("internal error details leaked: %s", got)
	}
}

func TestErrorEnvelopeFromAppErrorBridge(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)

	appErr := apperror.New(409, apperror.Conflict, "handle taken", nil)
	WriteError(rec, req, appErr)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	var body struct {
		Error *pkgerrors.HTTPError `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Error == nil || body.Error.Code != "CONFLICT" {
		t.Errorf("error = %+v, want CONFLICT", body.Error)
	}
}
