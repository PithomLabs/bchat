package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateNotification(ctx context.Context, create *store.Notification) (*store.Notification, error) {
	fields := []string{"initiator_id", "receiver_id", "ticket_url", "created_ts", "is_read"}
	placeholder := []string{"$1", "$2", "$3", "$4", "$5"}
	args := []any{create.InitiatorID, create.ReceiverID, create.TicketURL, create.CreatedTs, create.IsRead}

	stmt := "INSERT INTO notifications (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING id"
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(&create.ID); err != nil {
		return nil, err
	}
	return create, nil
}

func (d *DB) ListNotifications(ctx context.Context, find *store.FindNotification) ([]*store.Notification, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.ID != nil {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, *find.ID)
	}
	if find.ReceiverID != nil {
		where, args = append(where, fmt.Sprintf("receiver_id = $%d", len(args)+1)), append(args, *find.ReceiverID)
	}
	if find.IsRead != nil {
		where, args = append(where, fmt.Sprintf("is_read = $%d", len(args)+1)), append(args, *find.IsRead)
	}

	query := "SELECT id, initiator_id, receiver_id, ticket_url, created_ts, is_read FROM notifications WHERE " + strings.Join(where, " AND ") + " ORDER BY created_ts DESC"
	if find.Limit != nil {
		query = fmt.Sprintf("%s LIMIT %d", query, *find.Limit)
		if find.Offset != nil {
			query = fmt.Sprintf("%s OFFSET %d", query, *find.Offset)
		}
	}
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []*store.Notification{}
	for rows.Next() {
		notification := &store.Notification{}
		if err := rows.Scan(
			&notification.ID,
			&notification.InitiatorID,
			&notification.ReceiverID,
			&notification.TicketURL,
			&notification.CreatedTs,
			&notification.IsRead,
		); err != nil {
			return nil, err
		}
		list = append(list, notification)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return list, nil
}

func (d *DB) UpdateNotification(ctx context.Context, update *store.UpdateNotification) (*store.Notification, error) {
	set, args := []string{}, []any{}
	if update.IsRead != nil {
		set, args = append(set, fmt.Sprintf("is_read = $%d", len(args)+1)), append(args, *update.IsRead)
	}
	args = append(args, update.ID)
	query := "UPDATE notifications SET " + strings.Join(set, ", ") + fmt.Sprintf(" WHERE id = $%d RETURNING id, initiator_id, receiver_id, ticket_url, created_ts, is_read", len(args))

	notification := &store.Notification{}
	if err := d.db.QueryRowContext(ctx, query, args...).Scan(
		&notification.ID,
		&notification.InitiatorID,
		&notification.ReceiverID,
		&notification.TicketURL,
		&notification.CreatedTs,
		&notification.IsRead,
	); err != nil {
		return nil, err
	}
	return notification, nil
}
