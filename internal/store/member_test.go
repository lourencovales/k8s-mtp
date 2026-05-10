package store

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAddMember(t *testing.T) {
	db, mock := newTestDB(t)
	defer db.Close()

	mock.ExpectExec(`INSERT INTO tenant_members`).
		WithArgs("tenant-1", "user-1", "admin").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := db.AddMember(context.Background(), "tenant-1", "user-1", "admin"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRemoveMember(t *testing.T) {
	db, mock := newTestDB(t)
	defer db.Close()

	mock.ExpectExec(`DELETE FROM tenant_members`).
		WithArgs("tenant-1", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := db.RemoveMember(context.Background(), "tenant-1", "user-1"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestListMembers(t *testing.T) {
	db, mock := newTestDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT user_id, role, created_at`).
		WithArgs("tenant-1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "role", "created_at"}).
			AddRow("user-1", "admin", time.Now()).
			AddRow("user-2", "operator", time.Now()))

	members, err := db.ListMembers(context.Background(), "tenant-1")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
	if members[0].UserID != "user-1" || members[0].Role != "admin" {
		t.Errorf("expected user-1 and admin as fields for the first member")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestGetMemberRole(t *testing.T) {
	db, mock := newTestDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT role FROM tenant_members`).
		WithArgs("tenant-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("admin"))

	role, err := db.GetMemberRole(context.Background(), "tenant-1", "user-1")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if role != "admin" {
		t.Errorf("expected role to equal admin, got %s", role)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
