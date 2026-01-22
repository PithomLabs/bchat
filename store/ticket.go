package store

import (
	"context"
	"errors"
)

type TicketStatus string

const (
	TicketStatusOpen       TicketStatus = "OPEN"
	TicketStatusInProgress TicketStatus = "IN_PROGRESS"
	TicketStatusClosed     TicketStatus = "CLOSED"
)

type TicketPriority string

const (
	TicketPriorityLow    TicketPriority = "LOW"
	TicketPriorityMedium TicketPriority = "MEDIUM"
	TicketPriorityHigh   TicketPriority = "HIGH"
)

type Ticket struct {
	ID          int32
	Title       string
	Description string
	Status      TicketStatus
	Priority    TicketPriority
	CreatorID   int32
	AssigneeID  *int32
	CreatedTs   int64
	UpdatedTs   int64
	Type        string
	Tags        []string
}

type FindTicket struct {
	ID          *int32
	CreatorID   *int32
	Type        *string
	Description *string
}

type UpdateTicket struct {
	ID          int32
	Title       *string
	Description *string
	Status      *TicketStatus
	Priority    *TicketPriority
	AssigneeID  *int32
	UpdatedTs   *int64
	Type        *string
	Tags        []string
}

type DeleteTicket struct {
	ID int32
}

func (t *Ticket) Validate() error {
	if t.Title == "" {
		return errors.New("title is required")
	}
	if t.Status == "" {
		t.Status = TicketStatusOpen
	}
	if t.Priority == "" {
		t.Priority = TicketPriorityMedium
	}
	if len(t.Description) < 3 || t.Description[:3] != "/m/" {
		return errors.New("description must be a valid memo link starting with /m/")
	}
	return nil
}

type TicketStore interface {
	CreateTicket(ctx context.Context, ticket *Ticket) (*Ticket, error)
	ListTickets(ctx context.Context, find *FindTicket) ([]*Ticket, error)
	GetTicket(ctx context.Context, find *FindTicket) (*Ticket, error)
	UpdateTicket(ctx context.Context, update *UpdateTicket) (*Ticket, error)
	DeleteTicket(ctx context.Context, delete *DeleteTicket) error
}

func (s *Store) CreateTicket(ctx context.Context, ticket *Ticket) (*Ticket, error) {
	return s.driver.CreateTicket(ctx, ticket)
}

func (s *Store) ListTickets(ctx context.Context, find *FindTicket) ([]*Ticket, error) {
	return s.driver.ListTickets(ctx, find)
}

func (s *Store) GetTicket(ctx context.Context, find *FindTicket) (*Ticket, error) {
	return s.driver.GetTicket(ctx, find)
}

func (s *Store) UpdateTicket(ctx context.Context, update *UpdateTicket) (*Ticket, error) {
	return s.driver.UpdateTicket(ctx, update)
}

func (s *Store) DeleteTicket(ctx context.Context, delete *DeleteTicket) error {
	return s.driver.DeleteTicket(ctx, delete)
}
