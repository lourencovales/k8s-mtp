package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"git.assilvestrar.club/lourenco/k8s-mtp/internal/store"
)

func newTestHandler(t *testing.T) (*TenantHandler, sqlmock.Sqlmock) {
	t.Helper()
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	db := store.NewTestDB(rawDB, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := NewTenantHandler(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return h, mock
}

func TestListTenants_Success(t *testing.T) {
	h, mock := newTestHandler(t)

	mock.ExpectQuery(`SELECT.* FROM tenants.*WHERE deleted_at IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "namespace", "tier", "owner_email"}).
			AddRow("uid-1", "test-tenant", "tenant-test-tenant", "free", "test@test.com"))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ListTenants()(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %v", w.Result().Status)
	}

	var tenants []TenantResponse
	json.NewDecoder(w.Body).Decode(&tenants)
	if len(tenants) != 1 {
		t.Errorf("expected 1 tenant, got %d", len(tenants))
	}
	if tenants[0].Name != "test-tenant" {
		t.Errorf("expected tenant name to be `test-tenant`, instead got %s", tenants[0].Name)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCreateTenant_Valid(t *testing.T) {
	h, mock := newTestHandler(t)

	mock.ExpectQuery(`INSERT INTO tenants`).
		WithArgs(sqlmock.AnyArg(), "my-tenant", "tenant-my-tenant", "free", "owner@test.com").
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))

	body := strings.NewReader(`{"name":"my-tenant","tier":"free","owner_email":"owner@test.com"}`)
	req := httptest.NewRequest("POST", "/", body)
	w := httptest.NewRecorder()
	h.CreateTenant()(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	var resp TenantResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "my-tenant" {
		t.Errorf("expected name to be `my-tenant`, got %s", resp.Name)
	}
	if resp.Tier != "free" {
		t.Errorf("expected tier to be `free`, got %s", resp.Tier)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCreateTenant_InvalidTier(t *testing.T) {
	h, _ := newTestHandler(t)

	body := strings.NewReader(`{"name":"t","tier":"bad","owner_email":"bad@email.com"}`)
	req := httptest.NewRequest("POST", "/", body)
	w := httptest.NewRecorder()
	h.CreateTenant()(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var errResp ErrorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	if !strings.Contains(errResp.Error, "invalid tier") {
		t.Errorf("expected `invalid tier` error, got %q", errResp.Error)
	}
}

func TestCreateTenant_EmptyName(t *testing.T) {
	h, _ := newTestHandler(t)

	body := strings.NewReader(`{"name":""}`)
	req := httptest.NewRequest("POST", "/", body)
	w := httptest.NewRecorder()
	h.CreateTenant()(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var errResp ErrorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	if !strings.Contains(errResp.Error, "name is required") {
		t.Errorf("expected `tenant name is required` error, got %q", errResp.Error)
	}
}

func TestGetTenant_Found(t *testing.T) {
	h, mock := newTestHandler(t)

	mock.ExpectQuery(`SELECT id, name, namespace, tier, owner_email FROM tenants WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("uid-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "namespace", "tier", "owner_email"}).
			AddRow("uid-1", "found-tenant", "tenant-found-tenant", "enterprise", "e@t.com"))

	req := httptest.NewRequest("GET", "/", nil)
	req.SetPathValue("id", "uid-1")
	w := httptest.NewRecorder()
	h.GetTenant()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp TenantResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "found-tenant" {
		t.Errorf("expected `found-tenant` as name, got %s", resp.Name)
	}
	if resp.Tier != "enterprise" {
		t.Errorf("expected `enterprise` as tier, got %s", resp.Tier)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestGetTenant_NotFound(t *testing.T) {
	h, mock := newTestHandler(t)

	mock.ExpectQuery(`SELECT id, name, namespace, tier, owner_email FROM tenants WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("uid-1").
		WillReturnError(sql.ErrNoRows)

	req := httptest.NewRequest("GET", "/", nil)
	req.SetPathValue("id", "uid-1")
	w := httptest.NewRecorder()
	h.GetTenant()(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestUpdateTenant_Success(t *testing.T) {
	h, mock := newTestHandler(t)

	mock.ExpectQuery(`SELECT id, name, namespace, tier, owner_email FROM tenants WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("uid-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "namespace", "tier", "owner_email"}).
			AddRow("uid-1", "old-name", "ns", "free", "old@t.com"))

	mock.ExpectExec(`UPDATE tenants`).
		WithArgs("pro", "old@t.com", "uid-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	body := strings.NewReader(`{"tier":"pro"}`)
	req := httptest.NewRequest("PUT", "/", body)
	req.SetPathValue("id", "uid-1")
	w := httptest.NewRecorder()
	h.UpdateTenant()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp TenantResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Tier != "pro" {
		t.Errorf("expected tier to be `pro`, instead got %s", resp.Tier)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestDeleteTenant_Success(t *testing.T) {
	h, mock := newTestHandler(t)

	mock.ExpectExec(`UPDATE tenants`).
		WithArgs("uid-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	req := httptest.NewRequest("DELETE", "/", nil)
	req.SetPathValue("id", "uid-1")
	w := httptest.NewRecorder()
	h.DeleteTenant()(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
