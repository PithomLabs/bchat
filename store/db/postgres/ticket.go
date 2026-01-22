package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateTicket(ctx context.Context, create *store.Ticket) (*store.Ticket, error) {
	stmt := `
		INSERT INTO tickets (
			title,
			description,
			status,
			priority,
			creator_id,
			assignee_id,
			created_ts,
			updated_ts
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	if err := d.db.QueryRowContext(
		ctx,
		stmt,
		create.Title,
		create.Description,
		create.Status,
		create.Priority,
		create.CreatorID,
		create.AssigneeID,
		create.CreatedTs,
		create.UpdatedTs,
	).Scan(&create.ID); err != nil {
		return nil, err
	}

	return create, nil
}

func (d *DB) ListTickets(ctx context.Context, find *store.FindTicket) ([]*store.Ticket, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.ID != nil {
		where = append(where, "id = $1") // This logic is too simple for multiple args in postgres, need manual placeholder counting
	}
	// Fixing placeholder logic for Postgres
	where = []string{"1=1"}
	args = []interface{}{}
	argCounter := 1

	if find.ID != nil {
		where = append(where, fmt.Sprintf("id = $%d", argCounter))
		args = append(args, *find.ID)
		argCounter++
	}
	if find.CreatorID != nil {
		where = append(where, fmt.Sprintf("creator_id = $%d", argCounter))
		args = append(args, *find.CreatorID)
		argCounter++
	}
	if find.Description != nil {
		where = append(where, fmt.Sprintf("description = $%d", argCounter))
		args = append(args, *find.Description)
		argCounter++
	}

	query := fmt.Sprintf(`
		SELECT
			id,
			title,
			description,
			status,
			priority,
			creator_id,
			assignee_id,
			created_ts,
			updated_ts
		FROM tickets
		WHERE %s
		ORDER BY created_ts DESC
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*store.Ticket, 0)
	for rows.Next() {
		var ticket store.Ticket
		if err := rows.Scan(
			&ticket.ID,
			&ticket.Title,
			&ticket.Description,
			&ticket.Status,
			&ticket.Priority,
			&ticket.CreatorID,
			&ticket.AssigneeID,
			&ticket.CreatedTs,
			&ticket.UpdatedTs,
		); err != nil {
			return nil, err
		}
		list = append(list, &ticket)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return list, nil
}

func (d *DB) GetTicket(ctx context.Context, find *store.FindTicket) (*store.Ticket, error) {
	list, err := d.ListTickets(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

func (d *DB) UpdateTicket(ctx context.Context, update *store.UpdateTicket) (*store.Ticket, error) {
	set, args := []string{}, []interface{}{}
	argCounter := 1

	if update.Title != nil {
		set = append(set, fmt.Sprintf("title = $%d", argCounter))
		args = append(args, *update.Title)
		argCounter++
	}
	if update.Description != nil {
		set = append(set, fmt.Sprintf("description = $%d", argCounter))
		args = append(args, *update.Description)
		argCounter++
	}
	if update.Status != nil {
		set = append(set, fmt.Sprintf("status = $%d", argCounter))
		args = append(args, *update.Status)
		argCounter++
	}
	if update.Priority != nil {
		set = append(set, fmt.Sprintf("priority = $%d", argCounter))
		args = append(args, *update.Priority)
		argCounter++
	}
	if update.AssigneeID != nil {
		set = append(set, fmt.Sprintf("assignee_id = $%d", argCounter))
		args = append(args, *update.AssigneeID)
		argCounter++
	}
	if update.UpdatedTs != nil {
		set = append(set, fmt.Sprintf("updated_ts = $%d", argCounter))
		args = append(args, *update.UpdatedTs)
		argCounter++
	}

	args = append(args, update.ID)
	stmt := fmt.Sprintf(`
		UPDATE tickets
		SET %s
		WHERE id = $%d
		RETURNING id, title, description, status, priority, creator_id, assignee_id, created_ts, updated_ts
	`, strings.Join(set, ", "), argCounter)

	var ticket store.Ticket
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(
		&ticket.ID,
		&ticket.Title,
		&ticket.Description,
		&ticket.Status,
		&ticket.Priority,
		&ticket.CreatorID,
		&ticket.AssigneeID,
		&ticket.CreatedTs,
		&ticket.UpdatedTs,
	); err != nil {
		return nil, err
	}

	return &ticket, nil
}

func (d *DB) DeleteTicket(ctx context.Context, delete *store.DeleteTicket) error {
	stmt := `DELETE FROM tickets WHERE id = $1`
	result, err := d.db.ExecContext(ctx, stmt, delete.ID)
	if err != nil {
		return err
	}
	if _, err := result.RowsAffected(); err != nil {
		return err
	}
	return nil
}
