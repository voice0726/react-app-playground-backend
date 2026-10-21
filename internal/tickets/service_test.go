package tickets_test

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/tickets"
	"github.com/voice0726/react-app-playground-backend/internal/tickets/tickettest"
)

var (
	alice = tickets.Actor{ID: "user-a", Name: "Alice", Email: "alice@example.com"}
	bob   = tickets.Actor{ID: "user-b", Name: "Bob", Email: "bob@example.com"}
	orgA  = uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	orgB  = uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
)

func ptr(s string) *string { return &s }

func optional(s *string) tickets.OptionalString { return tickets.OptionalString{Set: true, Value: s} }

func newService() *tickets.Service { return tickets.NewService(tickettest.NewRepository()) }

func mustCreate(t *testing.T, s *tickets.Service, actor tickets.Actor, title string) tickets.Ticket {
	t.Helper()
	created, err := s.Create(context.Background(), orgA, actor, tickets.CreateParams{Title: title})
	if err != nil {
		t.Fatal(err)
	}
	return created
}

func idOf(t tickets.Ticket) string { return strconv.FormatUint(t.ID, 10) }

func TestCreateNormalizesAndRecordsActor(t *testing.T) {
	s := newService()
	created, err := s.Create(context.Background(), orgA, alice, tickets.CreateParams{Title: "  Fix  ", Assignee: ptr("  bob ")})
	if err != nil {
		t.Fatal(err)
	}
	if created.Title != "Fix" || created.Status != tickets.StatusOpen || *created.Assignee != "bob" {
		t.Fatalf("unexpected ticket: %+v", created)
	}
	if created.CreatedBy != alice || created.UpdatedBy != alice {
		t.Fatalf("actor not recorded: %+v / %+v", created.CreatedBy, created.UpdatedBy)
	}
	if len(created.History) != 0 || len(created.Comments) != 0 {
		t.Fatalf("new ticket must have empty history/comments")
	}
}

func TestCreateValidation(t *testing.T) {
	s := newService()
	for name, tc := range map[string]struct {
		params tickets.CreateParams
		want   error
	}{
		"blank title":    {tickets.CreateParams{Title: "  "}, tickets.ErrInvalidTitle},
		"invalid status": {tickets.CreateParams{Title: "x", Status: "pending"}, tickets.ErrInvalidStatus},
		"all status":     {tickets.CreateParams{Title: "x", Status: "all"}, tickets.ErrInvalidStatus},
	} {
		if _, err := s.Create(context.Background(), orgA, alice, tc.params); !errors.Is(err, tc.want) {
			t.Errorf("%s: err=%v, want %v", name, err, tc.want)
		}
	}
	got, err := s.Create(context.Background(), orgA, alice, tickets.CreateParams{Title: "x", Status: " CLOSED "})
	if err != nil || got.Status != tickets.StatusClosed {
		t.Fatalf("status should be normalized: %+v %v", got, err)
	}
}

func TestReplaceRecordsStatusHistoryOnlyOnChange(t *testing.T) {
	s := newService()
	created := mustCreate(t, s, alice, "A")
	ctx := context.Background()

	same, err := s.Replace(ctx, orgA, alice, idOf(created), tickets.ReplaceParams{Title: "B", Status: "open"})
	if err != nil || len(same.History) != 0 || same.Title != "B" {
		t.Fatalf("title-only replace: %+v %v", same, err)
	}
	changed, err := s.Replace(ctx, orgA, alice, idOf(created), tickets.ReplaceParams{Title: "B", Status: "closed", Assignee: ptr("  ")})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Assignee != nil || len(changed.History) != 1 {
		t.Fatalf("unexpected: %+v", changed)
	}
	item := changed.History[0]
	if item.OperationID == "" || item.Actor != alice || len(item.Changes) != 1 {
		t.Fatalf("bad history item: %+v", item)
	}
	if c := item.Changes[0]; c.Field != tickets.HistoryFieldStatus || *c.Before != "open" || *c.After != "closed" {
		t.Fatalf("bad change: %+v", c)
	}
	if _, err := s.Replace(ctx, orgA, alice, idOf(created), tickets.ReplaceParams{Title: "B"}); !errors.Is(err, tickets.ErrInvalidStatus) {
		t.Fatalf("replace requires status: %v", err)
	}
	if _, err := s.Replace(ctx, orgA, alice, "999", tickets.ReplaceParams{Title: "B", Status: "open"}); !errors.Is(err, tickets.ErrNotFound) {
		t.Fatalf("missing ticket: %v", err)
	}
}

func TestPatch(t *testing.T) {
	s := newService()
	ctx := context.Background()
	created, err := s.Create(ctx, orgA, alice, tickets.CreateParams{Title: "A", Assignee: ptr("bob")})
	if err != nil {
		t.Fatal(err)
	}
	id := idOf(created)

	if _, err := s.Patch(ctx, orgA, alice, id, tickets.PatchParams{}); !errors.Is(err, tickets.ErrEmptyPatch) {
		t.Fatalf("empty patch: %v", err)
	}
	if _, err := s.Patch(ctx, orgA, alice, id, tickets.PatchParams{Title: optional(nil)}); !errors.Is(err, tickets.ErrInvalidTitle) {
		t.Fatalf("null title: %v", err)
	}
	if _, err := s.Patch(ctx, orgA, alice, id, tickets.PatchParams{Status: optional(ptr("bogus"))}); !errors.Is(err, tickets.ErrInvalidStatus) {
		t.Fatalf("bad status: %v", err)
	}
	cleared, err := s.Patch(ctx, orgA, alice, id, tickets.PatchParams{Assignee: optional(nil)})
	if err != nil || cleared.Assignee != nil || len(cleared.History) != 0 {
		t.Fatalf("assignee null: %+v %v", cleared, err)
	}
	closed, err := s.Patch(ctx, orgB, bob, id, tickets.PatchParams{Status: optional(ptr("closed"))})
	if !errors.Is(err, tickets.ErrNotFound) {
		t.Fatalf("other organization must not patch: %+v %v", closed, err)
	}
	closed, err = s.Patch(ctx, orgA, alice, id, tickets.PatchParams{Status: optional(ptr("closed"))})
	if err != nil || closed.Status != tickets.StatusClosed || len(closed.History) != 1 {
		t.Fatalf("status patch: %+v %v", closed, err)
	}
}

func TestOtherOrganizationsTicketsAreInvisible(t *testing.T) {
	s := newService()
	ctx := context.Background()
	created := mustCreate(t, s, alice, "secret")
	id := idOf(created)

	if _, err := s.Find(ctx, orgB, id); !errors.Is(err, tickets.ErrNotFound) {
		t.Fatalf("find: %v", err)
	}
	if err := s.Delete(ctx, orgB, id); !errors.Is(err, tickets.ErrNotFound) {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.AddComment(ctx, orgB, bob, id, "hi"); !errors.Is(err, tickets.ErrNotFound) {
		t.Fatalf("comment: %v", err)
	}
	list, err := s.List(ctx, orgB, tickets.NormalizeListQuery(url.Values{}))
	if err != nil || list.Total != 0 || len(list.Items) != 0 {
		t.Fatalf("list: %+v %v", list, err)
	}
	if _, err := s.Find(ctx, orgA, id); err != nil {
		t.Fatal(err)
	}
}

func TestMembersShareOrganizationTickets(t *testing.T) {
	s := newService()
	ctx := context.Background()
	id := idOf(mustCreate(t, s, alice, "shared"))

	if _, err := s.Find(ctx, orgA, id); err != nil {
		t.Fatal(err)
	}
	patched, err := s.Patch(ctx, orgA, bob, id, tickets.PatchParams{Status: optional(ptr("closed"))})
	if err != nil || patched.CreatedBy != alice || patched.UpdatedBy != bob || patched.OwnerUserID != alice.ID {
		t.Fatalf("member patch: %+v %v", patched, err)
	}
	commented, err := s.AddComment(ctx, orgA, bob, id, "from bob")
	if err != nil || commented.Comments[0].CreatedBy != bob {
		t.Fatalf("member comment: %+v %v", commented, err)
	}
	commentID := strconv.FormatUint(commented.Comments[0].ID, 10)
	if _, err := s.UpdateComment(ctx, orgA, alice, id, commentID, "x"); !errors.Is(err, tickets.ErrCommentPermissionDenied) {
		t.Fatalf("other member's comment: %v", err)
	}
}

func TestCommentLifecycleAndAuthorship(t *testing.T) {
	s := newService()
	ctx := context.Background()
	id := idOf(mustCreate(t, s, alice, "A"))

	added, err := s.AddComment(ctx, orgA, alice, id, "  first ")
	if err != nil || len(added.Comments) != 1 || added.Comments[0].Body != "first" || added.Comments[0].CreatedBy != alice {
		t.Fatalf("add: %+v %v", added, err)
	}
	commentID := strconv.FormatUint(added.Comments[0].ID, 10)
	if _, err := s.AddComment(ctx, orgA, alice, id, " "); !errors.Is(err, tickets.ErrInvalidCommentBody) {
		t.Fatalf("blank body: %v", err)
	}

	updated, err := s.UpdateComment(ctx, orgA, alice, id, commentID, "edited")
	if err != nil || updated.Comments[0].Body != "edited" {
		t.Fatalf("update: %+v %v", updated, err)
	}
	if _, err := s.UpdateComment(ctx, orgA, alice, id, "999", "x"); !errors.Is(err, tickets.ErrCommentNotFound) {
		t.Fatalf("missing comment: %v", err)
	}
	if _, err := s.UpdateComment(ctx, orgA, alice, id, "abc", "x"); !errors.Is(err, tickets.ErrCommentNotFound) {
		t.Fatalf("bad comment id: %v", err)
	}
	deleted, err := s.DeleteComment(ctx, orgA, alice, id, commentID)
	if err != nil || len(deleted.Comments) != 0 {
		t.Fatalf("delete: %+v %v", deleted, err)
	}
}

func TestCommentByAnotherAuthorIsForbidden(t *testing.T) {
	repo := tickettest.NewRepository()
	s := tickets.NewService(repo)
	ctx := context.Background()
	created := mustCreate(t, s, alice, "A")
	withComment, err := repo.AddComment(ctx, orgA, created.ID, alice, func() time.Time { return created.UpdatedAt }, tickets.Comment{Body: "by bob", CreatedBy: bob})
	if err != nil {
		t.Fatal(err)
	}
	id, commentID := idOf(created), strconv.FormatUint(withComment.Comments[0].ID, 10)
	if _, err := s.UpdateComment(ctx, orgA, alice, id, commentID, "x"); !errors.Is(err, tickets.ErrCommentPermissionDenied) {
		t.Fatalf("update: %v", err)
	}
	if _, err := s.DeleteComment(ctx, orgA, alice, id, commentID); !errors.Is(err, tickets.ErrCommentPermissionDenied) {
		t.Fatalf("delete: %v", err)
	}
}

func TestListFilterSortPaginate(t *testing.T) {
	s := newService()
	ctx := context.Background()
	for _, title := range []string{"Login page", "Refactor", "LOGIN telemetry", "Dark mode", "Typo"} {
		mustCreate(t, s, alice, title)
	}
	if _, err := s.Patch(ctx, orgA, alice, "2", tickets.PatchParams{Status: optional(ptr("closed"))}); err != nil {
		t.Fatal(err)
	}
	titles := func(q url.Values) ([]string, int) {
		res, err := s.List(ctx, orgA, tickets.NormalizeListQuery(q))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, it := range res.Items {
			out = append(out, it.Title)
		}
		return out, res.Total
	}
	if got, total := titles(url.Values{"q": {"login"}}); !reflect.DeepEqual(got, []string{"Login page", "LOGIN telemetry"}) || total != 2 {
		t.Fatalf("q: %v %d", got, total)
	}
	if got, total := titles(url.Values{"status": {"closed"}}); !reflect.DeepEqual(got, []string{"Refactor"}) || total != 1 {
		t.Fatalf("status: %v %d", got, total)
	}
	if got, _ := titles(url.Values{"sortBy": {"id"}, "sortOrder": {"dsc"}, "pageSize": {"2"}}); !reflect.DeepEqual(got, []string{"Typo", "Dark mode"}) {
		t.Fatalf("sort: %v", got)
	}
	if got, total := titles(url.Values{"pageSize": {"2"}, "page": {"3"}}); !reflect.DeepEqual(got, []string{"Typo"}) || total != 5 {
		t.Fatalf("page: %v %d", got, total)
	}
	if got, _ := titles(url.Values{"page": {"9223372036854775807"}, "pageSize": {"2"}}); len(got) != 0 {
		t.Fatalf("overflowing page: %v", got)
	}
	if got, total := titles(url.Values{"pageSize": {"9223372036854775807"}}); len(got) != 5 || total != 5 {
		t.Fatalf("huge pageSize: %v %d", got, total)
	}
	if got, _ := titles(url.Values{"page": {"9"}}); len(got) != 0 {
		t.Fatalf("page beyond end: %v", got)
	}
	if got, total := titles(url.Values{"status": {"bogus"}, "page": {"-1"}, "pageSize": {"x"}, "sortOrder": {"zzz"}}); total != 5 || got[0] != "Login page" {
		t.Fatalf("fallback: %v %d", got, total)
	}
}

func TestInvalidIDsAreNotFound(t *testing.T) {
	s := newService()
	for _, id := range []string{"", "abc", "0", "-1", "99999999999999999999", "9223372036854775808"} {
		if _, err := s.Find(context.Background(), orgA, id); !errors.Is(err, tickets.ErrNotFound) {
			t.Errorf("id %q: %v", id, err)
		}
	}
}
