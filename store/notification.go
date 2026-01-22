package store

import (
	"context"
)

type Notification struct {
	ID          int32
	InitiatorID int32
	ReceiverID  int32
	TicketURL   string
	CreatedTs   int64
	IsRead      bool
}

type FindNotification struct {
	ID         *int32
	ReceiverID *int32
	IsRead     *bool
	Limit      *int
	Offset     *int
}

type UpdateNotification struct {
	ID     int32
	IsRead *bool
}

func (s *Store) CreateNotification(ctx context.Context, create *Notification) (*Notification, error) {
	return s.driver.CreateNotification(ctx, create)
}

func (s *Store) ListNotifications(ctx context.Context, find *FindNotification) ([]*Notification, error) {
	return s.driver.ListNotifications(ctx, find)
}

func (s *Store) UpdateNotification(ctx context.Context, update *UpdateNotification) (*Notification, error) {
	return s.driver.UpdateNotification(ctx, update)
}
