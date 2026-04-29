package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "git.assilvestrar.club/lourenco/k8s-mtp/pkg/api/v1"
)

func newTestDB(t *testing.T) (*Database, sqlmock.Sqlmock) {
	t.Helper()
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	return &Database{
		db:     rawDB,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, mock
}

func newTestTenant() *v1.Tenant {
	return &v1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("this-is-a-test-UID"),
		},
		Spec: v1.TenantSpec{
			Name:       "test",
			Tier:       v1.TierFree,
			OwnerEmail: "test@test.com",
		},
		Status: v1.TenantStatus{
			Namespace: "tenant-test",
		},
	}
}

func TestCreateTenant(t *testing.T) {
	db, mock := newTestDB(t)
	defer db.Close()

	mock.ExpectQuery(`INSERT INTO tenants`).
		WithArgs("this-is-a-test-UID", "test", "tenant-test", "free", "test@test.com").
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))

	tenant := newTestTenant()

	if err := db.CreateTenant(context.Background(), tenant); err != nil {
		t.Errorf("error creating tenant, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestGetTenantByID(t *testing.T) {
	db, mock := newTestDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, name, namespace, tier, owner_email FROM tenants WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("this-is-a-test-UID").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "namespace", "tier", "owner_email"}).
			AddRow("this-is-a-test-UID", "test", "tenant-test", "free", "test@test.com"))

	tenant := newTestTenant()

	getTenant, err := db.GetTenantByID(context.Background(), "this-is-a-test-UID")
	if err != nil {
		t.Errorf("error getting tenant by ID, got %v", err)
	}

	if getTenant.Spec.Name != tenant.Spec.Name ||
		getTenant.Spec.Tier != tenant.Spec.Tier ||
		getTenant.Spec.OwnerEmail != tenant.Spec.OwnerEmail ||
		getTenant.Status.Namespace != tenant.Status.Namespace {
		t.Errorf("error getting tenant by ID")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestListTenants(t *testing.T) {
	db, mock := newTestDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT.*FROM tenants.*WHERE deleted_at IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "namespace", "tier", "owner_email"}).
			AddRow("this-is-a-test-UID", "test", "tenant-test", "free", "test@test.com"))

	tenant := newTestTenant()

	listOfTenants, err := db.ListTenants(context.Background())
	if err != nil {
		t.Errorf("error listing tenant, got %v", err)
	}

	if len(listOfTenants) != 1 ||
		listOfTenants[0].Spec.Name != tenant.Spec.Name ||
		listOfTenants[0].Spec.Tier != tenant.Spec.Tier ||
		listOfTenants[0].Spec.OwnerEmail != tenant.Spec.OwnerEmail ||
		listOfTenants[0].Status.Namespace != tenant.Status.Namespace {
		t.Errorf("expected the fields to match")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestUpdateTenant(t *testing.T) {
	db, mock := newTestDB(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE tenants`).
		WithArgs("free", "test@test.com", "this-is-a-test-UID").
		WillReturnResult(sqlmock.NewResult(0, 1))

	tenant := newTestTenant()

	if err := db.UpdateTenant(context.Background(), tenant); err != nil {
		t.Errorf("expected update, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestDeleteTenant(t *testing.T) {
	db, mock := newTestDB(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE tenants`).
		WithArgs("this-is-a-test-UID").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := db.DeleteTenant(context.Background(), "this-is-a-test-UID"); err != nil {
		t.Errorf("expected update, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
