package handlers

import (
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

func newTestMemberHandler(t *testing.T) (*MemberHandler, sqlmock.Sqlmock) {
	t.Helper()
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}

	db := store.NewTestDB(rawDB, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := NewMemberHandler(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return h, mock
}

func TestAddMember_Success(t *testing.T) {
	h, mock := newTestMemberHandler(t)

	mock.ExpectQuery(`SELECT id, name, namespace, tier, owner_email FROM tenants WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("tenant-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "namespace", "tier", "owner_email"}).
			AddRow("tenant-1", "test", "ns", "free", "test@test.com"))

	mock.ExpectExec(`INSERT INTO tenant_members`).
		WithArgs("tenant-1", "user-1", "admin").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := strings.NewReader(`{"user_id":"user-1","role":"admin"}`)
	req := httptest.NewRequest("POST", "/", body)
	req.SetPathValue("id", "tenant-1")
	w := httptest.NewRecorder()
	h.AddMember()(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, instead got %d", w.Code)
	}

	var resp MemberResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.UserID != "user-1" {
		t.Errorf("expected user id to be user-1, instead got %s", resp.UserID)
	}
	if resp.Role != "admin" {
		t.Errorf("expected role to be admin, instead got %s", resp.Role)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRemoveMember_Success(t *testing.T) {
	h, mock := newTestMemberHandler(t)

	mock.ExpectExec(`DELETE FROM tenant_members`).
		WithArgs("tenant-1", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	req := httptest.NewRequest("DELETE", "/", nil)
	req.SetPathValue("id", "tenant-1")
	req.SetPathValue("userId", "user-1")
	w := httptest.NewRecorder()
	h.RemoveMember()(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, instead got %d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestListMembers_Success(t *testing.T) {
	h, mock := newTestMemberHandler(t)

	mock.ExpectQuery(`SELECT user_id, role, created_at`).
		WithArgs("tenant-1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "role", "created_at"}).
			AddRow("user-1", "admin", time.Now()).
			AddRow("user-2", "operator", time.Now()))

	req := httptest.NewRequest("GET", "/", nil)
	req.SetPathValue("id", "tenant-1")
	w := httptest.NewRecorder()
	h.ListMembers()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp []MemberResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 members, got %d", len(resp))
	}
	if resp[0].UserID != "user-1" {
		t.Errorf("expected user id of the first member to be user-1, instead got %s", resp[0].UserID)
	}
	if resp[0].Role != "admin" {
		t.Errorf("expected role of the first member to be admin, instead got %s", resp[0].Role)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
