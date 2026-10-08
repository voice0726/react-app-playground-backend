package tickets

import (
	"cmp"
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
)

var (
	ErrNotFound                = httperror.New(httperror.NotFound, "ticket_not_found", "Ticket not found.", nil)
	ErrCommentNotFound         = httperror.New(httperror.NotFound, "comment_not_found", "Comment not found.", nil)
	ErrCommentPermissionDenied = httperror.New(httperror.Forbidden, "comment_forbidden", "Comment can only be modified by its author.", nil)
	ErrInvalidTitle            = httperror.New(httperror.Validation, "invalid_title", "Title is required.", nil)
	ErrInvalidStatus           = httperror.New(httperror.Validation, "invalid_status", "Status must be open or closed.", nil)
	ErrInvalidCommentBody      = httperror.New(httperror.Validation, "invalid_comment_body", "Comment body is required.", nil)
	ErrEmptyPatch              = httperror.New(httperror.Validation, "empty_patch", "At least one field must be provided.", nil)
)

type CreateParams struct {
	Title    string
	Status   Status
	Assignee *string
}

type ReplaceParams struct {
	Title    string
	Status   Status
	Assignee *string
}

type PatchParams struct {
	Title    OptionalString
	Status   OptionalString
	Assignee OptionalString
}

type Service struct {
	repo                Repository
	now                 func() time.Time
	generateOperationID func() (string, error)
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: currentTimestamp, generateOperationID: newOperationID}
}

func (s *Service) List(ctx context.Context, user string, query ListQuery) (ListResult, error) {
	items, err := s.repo.List(ctx, user)
	if err != nil {
		return ListResult{}, err
	}
	filtered := filterByQuery(filterByStatus(items, query.Status), query.Q)
	sortTickets(filtered, query.SortBy, query.SortOrder)
	return ListResult{Items: paginate(filtered, query.Page, query.PageSize), Total: len(filtered)}, nil
}

func (s *Service) Find(ctx context.Context, user string, id string) (Ticket, error) {
	parsed, err := parseID(id, ErrNotFound)
	if err != nil {
		return Ticket{}, err
	}
	return s.repo.Find(ctx, user, parsed)
}

func (s *Service) Create(ctx context.Context, actor Actor, p CreateParams) (Ticket, error) {
	title, ok := normalizeTitle(p.Title)
	if !ok {
		return Ticket{}, ErrInvalidTitle
	}
	status, ok := normalizeStatus(p.Status, StatusOpen)
	if !ok {
		return Ticket{}, ErrInvalidStatus
	}
	now := s.now()
	return s.repo.Create(ctx, actor.ID, Ticket{
		Title: title, Status: status, Assignee: normalizeAssignee(p.Assignee),
		CreatedBy: actor, UpdatedBy: actor, CreatedAt: now, UpdatedAt: now,
	})
}

func (s *Service) Replace(ctx context.Context, actor Actor, id string, p ReplaceParams) (Ticket, error) {
	title, ok := normalizeTitle(p.Title)
	if !ok {
		return Ticket{}, ErrInvalidTitle
	}
	status, ok := normalizeStatus(p.Status, "")
	if !ok {
		return Ticket{}, ErrInvalidStatus
	}
	existing, err := s.load(ctx, actor.ID, id)
	if err != nil {
		return Ticket{}, err
	}
	previous := existing.Status
	existing.Title, existing.Status, existing.Assignee = title, status, normalizeAssignee(p.Assignee)
	return s.save(ctx, actor, existing, previous)
}

func (s *Service) Patch(ctx context.Context, actor Actor, id string, p PatchParams) (Ticket, error) {
	existing, err := s.load(ctx, actor.ID, id)
	if err != nil {
		return Ticket{}, err
	}
	previous := existing.Status
	if p.Title.Set {
		title, ok := normalizeTitle(p.Title.String())
		if !ok {
			return Ticket{}, ErrInvalidTitle
		}
		existing.Title = title
	}
	if p.Status.Set {
		status, ok := normalizeStatus(Status(p.Status.String()), "")
		if !ok {
			return Ticket{}, ErrInvalidStatus
		}
		existing.Status = status
	}
	if p.Assignee.Set {
		existing.Assignee = normalizeAssignee(p.Assignee.Value)
	}
	if !p.Title.Set && !p.Status.Set && !p.Assignee.Set {
		return Ticket{}, ErrEmptyPatch
	}
	return s.save(ctx, actor, existing, previous)
}

func (s *Service) Delete(ctx context.Context, user string, id string) error {
	parsed, err := parseID(id, ErrNotFound)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, user, parsed)
}

func (s *Service) AddComment(ctx context.Context, actor Actor, id string, body string) (Ticket, error) {
	ticketID, err := parseID(id, ErrNotFound)
	if err != nil {
		return Ticket{}, err
	}
	text, ok := normalizeTitle(body)
	if !ok {
		return Ticket{}, ErrInvalidCommentBody
	}
	existing, err := s.repo.Find(ctx, actor.ID, ticketID)
	if err != nil {
		return Ticket{}, err
	}
	now := s.now()
	existing.UpdatedBy, existing.UpdatedAt = actor, now
	return s.repo.AddComment(ctx, actor.ID, existing, Comment{Body: text, CreatedBy: actor, CreatedAt: now})
}

func (s *Service) UpdateComment(ctx context.Context, actor Actor, id string, commentID string, body string) (Ticket, error) {
	ticketID, parsedComment, err := parseCommentIDs(id, commentID)
	if err != nil {
		return Ticket{}, err
	}
	text, ok := normalizeTitle(body)
	if !ok {
		return Ticket{}, ErrInvalidCommentBody
	}
	existing, comment, err := s.loadComment(ctx, actor, ticketID, parsedComment)
	if err != nil {
		return Ticket{}, err
	}
	existing.UpdatedBy, existing.UpdatedAt = actor, s.now()
	comment.Body = text
	return s.repo.UpdateComment(ctx, actor.ID, existing, comment)
}

func (s *Service) DeleteComment(ctx context.Context, actor Actor, id string, commentID string) (Ticket, error) {
	ticketID, parsedComment, err := parseCommentIDs(id, commentID)
	if err != nil {
		return Ticket{}, err
	}
	existing, comment, err := s.loadComment(ctx, actor, ticketID, parsedComment)
	if err != nil {
		return Ticket{}, err
	}
	existing.UpdatedBy, existing.UpdatedAt = actor, s.now()
	return s.repo.DeleteComment(ctx, actor.ID, existing, comment.ID)
}

func (s *Service) load(ctx context.Context, user string, id string) (Ticket, error) {
	parsed, err := parseID(id, ErrNotFound)
	if err != nil {
		return Ticket{}, err
	}
	return s.repo.Find(ctx, user, parsed)
}

// loadComment は ticket と対象 comment を取得し、操作者が作成者であることを確認する。
func (s *Service) loadComment(ctx context.Context, actor Actor, ticketID, parsedComment uint64) (Ticket, Comment, error) {
	existing, err := s.repo.Find(ctx, actor.ID, ticketID)
	if err != nil {
		return Ticket{}, Comment{}, err
	}
	for _, comment := range existing.Comments {
		if comment.ID != parsedComment {
			continue
		}
		if comment.CreatedBy.ID != actor.ID {
			return Ticket{}, Comment{}, ErrCommentPermissionDenied
		}
		return existing, comment, nil
	}
	return Ticket{}, Comment{}, ErrCommentNotFound
}

// save は更新者と更新時刻を設定し、status が変わっていれば履歴を付けて保存する。
func (s *Service) save(ctx context.Context, actor Actor, ticket Ticket, previous Status) (Ticket, error) {
	now := s.now()
	ticket.UpdatedBy, ticket.UpdatedAt = actor, now
	var history []HistoryItem
	if previous != ticket.Status {
		operationID, err := s.generateOperationID()
		if err != nil {
			return Ticket{}, err
		}
		history = buildStatusHistory(operationID, previous, ticket.Status, actor, now)
	}
	return s.repo.Replace(ctx, actor.ID, ticket, history)
}

func filterByStatus(items []Ticket, status Status) []Ticket {
	filtered := make([]Ticket, 0, len(items))
	for _, item := range items {
		if status == StatusAll || item.Status == status {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterByQuery(items []Ticket, q string) []Ticket {
	needle := strings.ToLower(strings.TrimSpace(q))
	if needle == "" {
		return items
	}
	filtered := make([]Ticket, 0, len(items))
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Title), needle) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func sortTickets(items []Ticket, sortBy SortBy, sortOrder SortOrder) {
	sort.SliceStable(items, func(i, j int) bool {
		compared := compareBy(items[i], items[j], sortBy)
		if compared == 0 {
			compared = cmp.Compare(items[i].ID, items[j].ID)
		}
		if sortOrder == SortOrderAsc {
			return compared < 0
		}
		return compared > 0
	})
}

func compareBy(left, right Ticket, sortBy SortBy) int {
	switch sortBy {
	case SortByID:
		return cmp.Compare(left.ID, right.ID)
	case SortByUpdatedAt:
		return left.UpdatedAt.Compare(right.UpdatedAt)
	default:
		return left.CreatedAt.Compare(right.CreatedAt)
	}
}

func paginate(items []Ticket, page, pageSize int) []Ticket {
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []Ticket{}
	}
	return items[start:min(start+pageSize, len(items))]
}

func normalizeTitle(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	return value, value != ""
}

func normalizeStatus(raw Status, fallback Status) (Status, bool) {
	switch value := Status(strings.ToLower(strings.TrimSpace(string(raw)))); value {
	case StatusOpen, StatusClosed:
		return value, true
	case "":
		return fallback, fallback != ""
	}
	return "", false
}

func normalizeAssignee(raw *string) *string {
	if raw == nil {
		return nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil
	}
	return &value
}

func parseCommentIDs(ticketID, commentID string) (uint64, uint64, error) {
	parsedTicket, err := parseID(ticketID, ErrNotFound)
	if err != nil {
		return 0, 0, err
	}
	parsedComment, err := parseID(commentID, ErrCommentNotFound)
	return parsedTicket, parsedComment, err
}

// parseID は bigint に収まる正の整数だけを受け付ける。
func parseID(raw string, notFound error) (uint64, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 63)
	if err != nil || parsed == 0 {
		return 0, notFound
	}
	return parsed, nil
}

func currentTimestamp() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }
