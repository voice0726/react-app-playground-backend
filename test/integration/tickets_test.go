//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strconv"
	"testing"

	"github.com/voice0726/react-app-playground-backend/internal/tickets"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ticketService(t *testing.T, db *sql.DB) *tickets.Service {
	t.Helper()
	ensureUp(t, db)
	if _, err := db.Exec(`DELETE FROM ticket_comments; DELETE FROM ticket_histories; DELETE FROM tickets`); err != nil {
		t.Fatal(err)
	}
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return tickets.NewService(tickets.NewRepository(gdb))
}

func idString(n uint64) string { return strconv.FormatUint(n, 10) }

func ptr(s string) *string { return &s }

func TestTicketsRepositoryLifecycleAndHistory(t *testing.T) {
	db := openDB(t)
	s := ticketService(t, db)
	ctx := context.Background()
	a := tickets.Actor{ID: "user-a", Name: "Alice", Email: "alice@example.com"}

	created, err := s.Create(ctx, a, tickets.CreateParams{Title: "Bug", Assignee: ptr("bob")})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.CreatedBy != a || created.UpdatedBy != a || len(created.History) != 0 {
		t.Fatalf("created=%+v", created)
	}
	id := idString(created.ID)

	closed, err := s.Patch(ctx, a, id, tickets.PatchParams{Status: tickets.OptionalString{Set: true, Value: ptr("closed")}})
	if err != nil || len(closed.History) != 1 || closed.History[0].Actor != a {
		t.Fatalf("closed=%+v err=%v", closed, err)
	}
	reopened, err := s.Replace(ctx, a, id, tickets.ReplaceParams{Title: "Bug", Status: "open"})
	if err != nil || len(reopened.History) != 2 {
		t.Fatalf("reopened=%+v err=%v", reopened, err)
	}
	if got := reopened.History[0].Changes[0]; *got.Before != "closed" || *got.After != "open" {
		t.Fatalf("history is not newest first: %+v", reopened.History)
	}

	withComment, err := s.AddComment(ctx, a, id, "first")
	if err != nil || len(withComment.Comments) != 1 || withComment.Comments[0].CreatedBy != a {
		t.Fatalf("add comment: %+v %v", withComment, err)
	}
	second, err := s.AddComment(ctx, a, id, "second")
	if err != nil || second.Comments[0].Body != "second" {
		t.Fatalf("comments must be newest first: %+v %v", second, err)
	}
	cid := idString(second.Comments[0].ID)
	edited, err := s.UpdateComment(ctx, a, id, cid, "edited")
	if err != nil || edited.Comments[0].Body != "edited" {
		t.Fatalf("update comment: %+v %v", edited, err)
	}
	removed, err := s.DeleteComment(ctx, a, id, cid)
	if err != nil || len(removed.Comments) != 1 {
		t.Fatalf("delete comment: %+v %v", removed, err)
	}
	if _, err := s.DeleteComment(ctx, a, id, cid); !errors.Is(err, tickets.ErrCommentNotFound) {
		t.Fatalf("second delete: %v", err)
	}

	list, err := s.List(ctx, a.ID, tickets.NormalizeListQuery(url.Values{}))
	if err != nil || list.Total != 1 || list.Items[0].ID != created.ID {
		t.Fatalf("list=%+v err=%v", list, err)
	}

	if err := s.Delete(ctx, a.ID, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Find(ctx, a.ID, id); !errors.Is(err, tickets.ErrNotFound) {
		t.Fatalf("find after delete: %v", err)
	}
	var rows int
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM ticket_comments) + (SELECT count(*) FROM ticket_histories)`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("children must cascade: rows=%d err=%v", rows, err)
	}
}

func TestTicketsCrossUserIsolation(t *testing.T) {
	db := openDB(t)
	s := ticketService(t, db)
	ctx := context.Background()
	a := tickets.Actor{ID: "user-a", Name: "Alice", Email: "alice@example.com"}
	b := tickets.Actor{ID: "user-b", Name: "Bob", Email: "bob@example.com"}

	created, err := s.Create(ctx, a, tickets.CreateParams{Title: "private"})
	if err != nil {
		t.Fatal(err)
	}
	id := idString(created.ID)
	if _, err := s.AddComment(ctx, a, id, "mine"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Find(ctx, b.ID, id); !errors.Is(err, tickets.ErrNotFound) {
		t.Fatalf("find: %v", err)
	}
	if _, err := s.Replace(ctx, b, id, tickets.ReplaceParams{Title: "x", Status: "closed"}); !errors.Is(err, tickets.ErrNotFound) {
		t.Fatalf("replace: %v", err)
	}
	if _, err := s.AddComment(ctx, b, id, "x"); !errors.Is(err, tickets.ErrNotFound) {
		t.Fatalf("add comment: %v", err)
	}
	if err := s.Delete(ctx, b.ID, id); !errors.Is(err, tickets.ErrNotFound) {
		t.Fatalf("delete: %v", err)
	}
	list, err := s.List(ctx, b.ID, tickets.NormalizeListQuery(url.Values{}))
	if err != nil || list.Total != 0 {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	got, err := s.Find(ctx, a.ID, id)
	if err != nil || got.Title != "private" || len(got.Comments) != 1 || len(got.History) != 0 {
		t.Fatalf("owner data was modified: %+v %v", got, err)
	}

	// 他ユーザーのチケットを参照する子行は複合外部キーで拒否される。
	_, err = db.Exec(`INSERT INTO ticket_comments (owner_user_id, ticket_id, body, created_by_id, created_by_name, created_by_email, created_at) VALUES ('user-b', $1, 'x', 'user-b', 'Bob', 'b@example.com', now())`, created.ID)
	requirePGCode(t, err, "23503")
	_, err = db.Exec(`INSERT INTO tickets (owner_user_id, title, status, created_by_id, created_by_name, created_by_email, updated_by_id, updated_by_name, updated_by_email, created_at, updated_at) VALUES ('user-a', 't', 'bogus', 'a', 'a', 'a', 'a', 'a', 'a', now(), now())`)
	requirePGCode(t, err, "23514")
}
