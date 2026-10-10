//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voice0726/react-app-playground-backend/internal/tickets"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ticketService(t *testing.T, db *sql.DB) *tickets.Service {
	t.Helper()
	return tickets.NewService(ticketRepository(t, db))
}

func ticketRepository(t *testing.T, db *sql.DB) *tickets.GORMRepository {
	t.Helper()
	ensureUp(t, db)
	if _, err := db.Exec(`DELETE FROM ticket_comments; DELETE FROM ticket_histories; DELETE FROM tickets`); err != nil {
		t.Fatal(err)
	}
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return tickets.NewRepository(gdb)
}

type gatedUpdateRepository struct {
	tickets.Repository
	firstEntered  chan struct{}
	continueFirst chan struct{}
	updates       atomic.Int32
}

func (r *gatedUpdateRepository) UpdateLocked(ctx context.Context, user string, id uint64, apply func(tickets.Ticket) (tickets.Ticket, []tickets.HistoryItem, error)) (tickets.Ticket, error) {
	switch r.updates.Add(1) {
	case 1:
		return r.Repository.UpdateLocked(ctx, user, id, func(current tickets.Ticket) (tickets.Ticket, []tickets.HistoryItem, error) {
			close(r.firstEntered)
			<-r.continueFirst
			return apply(current)
		})
	}
	return r.Repository.UpdateLocked(ctx, user, id, apply)
}

type gatedAddCommentRepository struct {
	tickets.Repository
	started chan struct{}
	once    sync.Once
}

func (r *gatedAddCommentRepository) AddComment(ctx context.Context, user string, id uint64, updatedBy tickets.Actor, now func() time.Time, comment tickets.Comment) (tickets.Ticket, error) {
	r.once.Do(func() { close(r.started) })
	return r.Repository.AddComment(ctx, user, id, updatedBy, now, comment)
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

func TestTicketsAssigneeNullClearsColumn(t *testing.T) {
	db := openDB(t)
	s := ticketService(t, db)
	ctx := context.Background()
	a := tickets.Actor{ID: "user-a", Name: "Alice", Email: "alice@example.com"}

	created, err := s.Create(ctx, a, tickets.CreateParams{Title: "Bug", Assignee: ptr("bob")})
	if err != nil || created.Assignee == nil {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	id := idString(created.ID)

	patched, err := s.Patch(ctx, a, id, tickets.PatchParams{Assignee: tickets.OptionalString{Set: true}})
	if err != nil || patched.Assignee != nil {
		t.Fatalf("patch null: %+v err=%v", patched, err)
	}
	var isNull bool
	if err := db.QueryRow(`SELECT assignee IS NULL FROM tickets WHERE id = $1`, created.ID).Scan(&isNull); err != nil || !isNull {
		t.Fatalf("assignee column must be NULL: isNull=%v err=%v", isNull, err)
	}

	if _, err := s.Replace(ctx, a, id, tickets.ReplaceParams{Title: "Bug", Status: "open", Assignee: ptr("carol")}); err != nil {
		t.Fatal(err)
	}
	replaced, err := s.Replace(ctx, a, id, tickets.ReplaceParams{Title: "Bug", Status: "open"})
	if err != nil || replaced.Assignee != nil {
		t.Fatalf("replace without assignee: %+v err=%v", replaced, err)
	}
	if err := db.QueryRow(`SELECT assignee IS NULL FROM tickets WHERE id = $1`, created.ID).Scan(&isNull); err != nil || !isNull {
		t.Fatalf("assignee column must be NULL after replace: isNull=%v err=%v", isNull, err)
	}
}

func TestTicketsOtherUsersCommentsAreNotFound(t *testing.T) {
	s := ticketService(t, openDB(t))
	ctx := context.Background()
	a := tickets.Actor{ID: "user-a", Name: "Alice", Email: "alice@example.com"}
	b := tickets.Actor{ID: "user-b", Name: "Bob", Email: "bob@example.com"}

	ticketA, err := s.Create(ctx, a, tickets.CreateParams{Title: "A's"})
	if err != nil {
		t.Fatal(err)
	}
	withComment, err := s.AddComment(ctx, a, idString(ticketA.ID), "A's comment")
	if err != nil {
		t.Fatal(err)
	}
	ticketAID, commentID := idString(ticketA.ID), idString(withComment.Comments[0].ID)
	ticketB, err := s.Create(ctx, b, tickets.CreateParams{Title: "B's"})
	if err != nil {
		t.Fatal(err)
	}

	// 他ユーザーのチケット ID 経由: チケットが見つからない。
	if _, err := s.UpdateComment(ctx, b, ticketAID, commentID, "hijack"); !errors.Is(err, tickets.ErrNotFound) {
		t.Fatalf("PUT via A's ticket: %v", err)
	}
	if _, err := s.DeleteComment(ctx, b, ticketAID, commentID); !errors.Is(err, tickets.ErrNotFound) {
		t.Fatalf("DELETE via A's ticket: %v", err)
	}
	// 自分のチケット ID に他ユーザーのコメント ID を組み合わせる: コメントが見つからない。
	if _, err := s.UpdateComment(ctx, b, idString(ticketB.ID), commentID, "hijack"); !errors.Is(err, tickets.ErrCommentNotFound) {
		t.Fatalf("PUT with A's comment id: %v", err)
	}
	if _, err := s.DeleteComment(ctx, b, idString(ticketB.ID), commentID); !errors.Is(err, tickets.ErrCommentNotFound) {
		t.Fatalf("DELETE with A's comment id: %v", err)
	}

	got, err := s.Find(ctx, a.ID, ticketAID)
	if err != nil || len(got.Comments) != 1 || got.Comments[0].Body != "A's comment" {
		t.Fatalf("A's comment was modified: %+v err=%v", got, err)
	}
}

func TestTicketsConcurrentIndependentPatchesAreMerged(t *testing.T) {
	db := openDB(t)
	s := ticketService(t, db)
	ctx := context.Background()
	actor := tickets.Actor{ID: "user-a", Name: "Alice", Email: "alice@example.com"}
	created, err := s.Create(ctx, actor, tickets.CreateParams{Title: "old", Assignee: ptr("old")})
	if err != nil {
		t.Fatal(err)
	}

	blocker, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback()
	var lockedID uint64
	if err := blocker.QueryRowContext(ctx, `SELECT id FROM tickets WHERE owner_user_id = $1 AND id = $2 FOR UPDATE`, actor.ID, created.ID).Scan(&lockedID); err != nil {
		t.Fatal(err)
	}

	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, patch := range []tickets.PatchParams{
		{Title: tickets.OptionalString{Set: true, Value: ptr("new")}},
		{Assignee: tickets.OptionalString{Set: true, Value: ptr("new")}},
	} {
		wg.Add(1)
		go func(patch tickets.PatchParams) {
			defer wg.Done()
			_, err := s.Patch(ctx, actor, idString(created.ID), patch)
			errs <- err
		}(patch)
	}
	waitForTicketLockWaiters(t, db, 2)
	if err := blocker.Commit(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.Find(ctx, actor.ID, idString(created.ID))
	if err != nil || got.Title != "new" || got.Assignee == nil || *got.Assignee != "new" {
		t.Fatalf("concurrent patches lost an update: ticket=%+v err=%v", got, err)
	}
}

func TestTicketsConcurrentCommentAndClosePreserveStatusHistory(t *testing.T) {
	db := openDB(t)
	updates := &gatedUpdateRepository{Repository: ticketRepository(t, db), firstEntered: make(chan struct{}), continueFirst: make(chan struct{})}
	repo := &gatedAddCommentRepository{Repository: updates, started: make(chan struct{})}
	s := tickets.NewService(repo)
	ctx := context.Background()
	actor := tickets.Actor{ID: "user-a", Name: "Alice", Email: "alice@example.com"}
	created, err := s.Create(ctx, actor, tickets.CreateParams{Title: "bug"})
	if err != nil {
		t.Fatal(err)
	}

	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	var release sync.Once
	releaseClose := func() { release.Do(func() { close(updates.continueFirst) }) }
	defer releaseClose()
	go func() {
		defer wg.Done()
		_, err := s.Patch(ctx, actor, idString(created.ID), tickets.PatchParams{Status: tickets.OptionalString{Set: true, Value: ptr("closed")}})
		errs <- err
	}()
	waitForSignal(t, updates.firstEntered, "close patch to acquire the ticket row")
	go func() {
		defer wg.Done()
		_, err := s.AddComment(ctx, actor, idString(created.ID), "triaged")
		errs <- err
	}()
	waitForSignal(t, repo.started, "comment operation to wait on the ticket row")
	waitForTicketLockWaiters(t, db, 1)
	releaseClose()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.Find(ctx, actor.ID, idString(created.ID))
	if err != nil || got.Status != tickets.StatusClosed || len(got.Comments) != 1 || got.Comments[0].Body != "triaged" {
		t.Fatalf("comment overwrote ticket state: ticket=%+v err=%v", got, err)
	}
	if len(got.History) != 1 || len(got.History[0].Changes) != 1 {
		t.Fatalf("history=%+v, want one status change", got.History)
	}
	change := got.History[0].Changes[0]
	if change.Field != tickets.HistoryFieldStatus || change.Before == nil || *change.Before != "open" || change.After == nil || *change.After != "closed" {
		t.Fatalf("status history=%+v", got.History)
	}
}

func waitForTicketLockWaiters(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		err := db.QueryRow(`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid() AND wait_event_type = 'Lock' AND query ILIKE '%tickets%'`).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d ticket row lock waiters", want)
}

func waitForSignal(t *testing.T, signal <-chan struct{}, operation string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", operation)
	}
}
