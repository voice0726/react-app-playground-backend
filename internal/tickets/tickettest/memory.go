// Package tickettest はテスト用のメモリ上の tickets.Repository を提供する。
package tickettest

import (
	"context"
	"slices"
	"sync"

	"github.com/voice0726/react-app-playground-backend/internal/tickets"
)

type entry struct {
	ticket   tickets.Ticket
	history  []tickets.HistoryItem
	comments []tickets.Comment
}

type Repository struct {
	mu        sync.Mutex
	nextID    uint64
	nextCmtID uint64
	entries   map[uint64]*entry
}

func NewRepository() *Repository { return &Repository{entries: map[uint64]*entry{}} }

func (r *Repository) get(user string, id uint64) (*entry, error) {
	e, ok := r.entries[id]
	if !ok || e.ticket.OwnerUserID != user {
		return nil, tickets.ErrNotFound
	}
	return e, nil
}

func (e *entry) snapshot() tickets.Ticket {
	t := e.ticket
	t.History = slices.Clone(e.history)
	slices.SortStableFunc(t.History, func(a, b tickets.HistoryItem) int { return b.ChangedAt.Compare(a.ChangedAt) })
	t.Comments = slices.Clone(e.comments)
	slices.Reverse(t.Comments)
	if t.History == nil {
		t.History = []tickets.HistoryItem{}
	}
	if t.Comments == nil {
		t.Comments = []tickets.Comment{}
	}
	return t
}

func (r *Repository) List(_ context.Context, user string) ([]tickets.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []tickets.Ticket
	for id := uint64(1); id <= r.nextID; id++ {
		if e, ok := r.entries[id]; ok && e.ticket.OwnerUserID == user {
			t := e.ticket
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *Repository) Find(_ context.Context, user string, id uint64) (tickets.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, err := r.get(user, id)
	if err != nil {
		return tickets.Ticket{}, err
	}
	return e.snapshot(), nil
}

func (r *Repository) Create(_ context.Context, user string, t tickets.Ticket) (tickets.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	t.ID, t.OwnerUserID = r.nextID, user
	e := &entry{ticket: t}
	r.entries[t.ID] = e
	return e.snapshot(), nil
}

func (r *Repository) update(user string, t tickets.Ticket) (*entry, error) {
	e, err := r.get(user, t.ID)
	if err != nil {
		return nil, err
	}
	t.OwnerUserID, t.CreatedBy, t.CreatedAt = user, e.ticket.CreatedBy, e.ticket.CreatedAt
	e.ticket = t
	return e, nil
}

func (r *Repository) Replace(_ context.Context, user string, t tickets.Ticket, history []tickets.HistoryItem) (tickets.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, err := r.update(user, t)
	if err != nil {
		return tickets.Ticket{}, err
	}
	e.history = append(e.history, history...)
	return e.snapshot(), nil
}

func (r *Repository) AddComment(_ context.Context, user string, t tickets.Ticket, c tickets.Comment) (tickets.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, err := r.update(user, t)
	if err != nil {
		return tickets.Ticket{}, err
	}
	r.nextCmtID++
	c.ID, c.OwnerUserID, c.TicketID = r.nextCmtID, user, t.ID
	e.comments = append(e.comments, c)
	return e.snapshot(), nil
}

func (e *entry) commentIndex(id uint64) int {
	return slices.IndexFunc(e.comments, func(c tickets.Comment) bool { return c.ID == id })
}

func (r *Repository) UpdateComment(_ context.Context, user string, t tickets.Ticket, c tickets.Comment) (tickets.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, err := r.get(user, t.ID)
	if err != nil {
		return tickets.Ticket{}, err
	}
	i := e.commentIndex(c.ID)
	if i < 0 {
		return tickets.Ticket{}, tickets.ErrCommentNotFound
	}
	e, _ = r.update(user, t)
	e.comments[i].Body = c.Body
	return e.snapshot(), nil
}

func (r *Repository) DeleteComment(_ context.Context, user string, t tickets.Ticket, commentID uint64) (tickets.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, err := r.get(user, t.ID)
	if err != nil {
		return tickets.Ticket{}, err
	}
	i := e.commentIndex(commentID)
	if i < 0 {
		return tickets.Ticket{}, tickets.ErrCommentNotFound
	}
	e, _ = r.update(user, t)
	e.comments = slices.Delete(e.comments, i, i+1)
	return e.snapshot(), nil
}

func (r *Repository) Delete(_ context.Context, user string, id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.get(user, id); err != nil {
		return err
	}
	delete(r.entries, id)
	return nil
}
