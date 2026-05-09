package store

import (
	"context"
	"database/sql"
	"time"
)

type Member struct {
	UserID    string
	Role      string
	CreatedAt time.Time
}

func (d *Database) AddMember(ctx context.Context, tenantID, userID, role string) error {
	query := `
	INSERT INTO tenant_members (tenant_id, user_id, role)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, user_id) DO UPDATE SET ROLE = $3
	`

	_, err := d.db.ExecContext(ctx, query, tenantID, userID, role)

	return err
}

func (d *Database) RemoveMember(ctx context.Context, tenantID, userID string) error {
	query := `
	DELETE FROM tenant_members
	WHERE tenant_id = $1 AND user_id = $2
	`

	_, err := d.db.ExecContext(ctx, query, tenantID, userID)

	return err
}

func (d *Database) ListMembers(ctx context.Context, tenantID string) ([]Member, error) {
	query := `
	SELECT user_id, role, created_at
	FROM tenant_members WHERE tenant_id = $1
	`

	rows, err := d.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		var member Member
		err := rows.Scan(
			&member.UserID,
			&member.Role,
			&member.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	return members, rows.Err()
}

func (d *Database) GetMemberRole(ctx context.Context, tenantID, userID string) (string, error) {
	query := `
	SELECT role FROM tenant_members
	WHERE tenant_id = $1 AND user_id = $2
	`

	var memberRole string
	err := d.db.QueryRowContext(ctx, query, tenantID, userID).Scan(&memberRole)
	if err == sql.ErrNoRows {
		return "", err
	}

	return memberRole, err
}
