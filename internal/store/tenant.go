package store

import (
	"context"
	"database/sql"

	"git.assilvestrar.club/lourenco/k8s-mtp/pkg/api/v1"
)

func (d *Database) CreateTenant(ctx context.Context, tenant *v1.Tenant) error {
	uid := string(tenant.UID)

	namespace := tenant.Status.Namespace

	query := `
	INSERT INTO tenants (id, name, namespace, tier, owner_email)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (id, name) DO NOTHING
	RETURNING created_at
	`

	err := d.db.QueryRowContext(
		ctx,
		query,
		uid,
		tenant.Spec.Name,
		namespace,
		string(tenant.Spec.Tier),
		tenant.Spec.OwnerEmail,
	).Scan(&tenant.CreationTimestamp.Time)

	if err == sql.ErrNoRows {
		return nil
	}

	return err
}

func (d *Database) GetTenantByID(ctx context.Context, id string) (*v1.Tenant, error) {
	query := `
	SELECT id, name, namespace, tier, owner_email
	FROM tenants
	WHERE id = $1 AND deleted_at IS NULL
	`

	tenant := &v1.Tenant{}

	err := d.db.QueryRowContext(ctx, query, id).Scan(
		&tenant.UID,
		&tenant.Spec.Name,
		&tenant.Status.Namespace,
		&tenant.Spec.Tier,
		&tenant.Spec.OwnerEmail,
	)

	if err == sql.ErrNoRows {
		return nil, nil // this is okay if tenant doesn't exist
	}

	return tenant, err
}

func (d *Database) GetTenantByName(ctx context.Context, name string) (*v1.Tenant, error) {
	query := `
	SELECT id, name, namespace, tier, owner_email
	FROM tenants
	WHERE name = $1 AND deleted_at IS NULL
	`

	tenant := &v1.Tenant{}

	err := d.db.QueryRowContext(ctx, query, name).Scan(
		&tenant.UID,
		&tenant.Spec.Name,
		&tenant.Status.Namespace,
		&tenant.Spec.Tier,
		&tenant.Spec.OwnerEmail,
	)

	if err == sql.ErrNoRows {
		return nil, err
	}

	return tenant, err
}

func (d *Database) ListTenants(ctx context.Context) ([]*v1.Tenant, error) {
	query := `
	SELECT id, name, namespace, tier, owner_email
	FROM tenants WHERE deleted_at IS NULL
	`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []*v1.Tenant
	for rows.Next() {
		tenant := &v1.Tenant{}
		err := rows.Scan(
			&tenant.UID,
			&tenant.Spec.Name,
			&tenant.Status.Namespace,
			&tenant.Spec.Tier,
			&tenant.Spec.OwnerEmail,
		)
		if err != nil {
			return nil, err
		}
		tenants = append(tenants, tenant)
	}

	return tenants, rows.Err()
}

func (d *Database) UpdateTenant(ctx context.Context, tenant *v1.Tenant) error {
	query := `
	UPDATE tenants
	SET tier = $1, owner_email = $2, updated_at = NOW()
	WHERE id = $3 AND deleted_at IS NULL
	`

	_, err := d.db.ExecContext(ctx, query,
		string(tenant.Spec.Tier),
		tenant.Spec.OwnerEmail,
		string(tenant.UID),
	)

	return err
}

func (d *Database) DeleteTenant(ctx context.Context, id string) error {
	query := `
	UPDATE tenants
	SET deleted_at = NOW()
	WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := d.db.ExecContext(ctx, query, id)

	return err
}

func (d *Database) HardDeleteTenant(ctx context.Context, id string) error {
	query := `
	DELETE FROM tenants WHERE id = $1
	`

	_, err := d.db.ExecContext(ctx, query, id)

	return err
}
