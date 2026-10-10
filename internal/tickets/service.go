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
	parsed, err := parseID(id, ErrNotFound)
	if err != nil {
		return Ticket{}, err
	}
	return s.repo.UpdateLocked(ctx, actor.ID, parsed, func(existing Ticket) (Ticket, []HistoryItem, error) {
		previous := existing.Status
		existing.Title, existing.Status, existing.Assignee = title, status, normalizeAssignee(p.Assignee)
		return s.prepareUpdate(actor, existing, previous)
	})
}

func (s *Service) Patch(ctx context.Context, actor Actor, id string, p PatchParams) (Ticket, error) {
	parsed, err := parseID(id, ErrNotFound)
	if err != nil {
		return Ticket{}, err
	}
	return s.repo.UpdateLocked(ctx, actor.ID, parsed, func(existing Ticket) (Ticket, []HistoryItem, error) {
		previous := existing.Status
		if p.Title.Set {
			title, ok := normalizeTitle(p.Title.String())
			if !ok {
				return Ticket{}, nil, ErrInvalidTitle
			}
			existing.Title = title
		}
		if p.Status.Set {
			status, ok := normalizeStatus(Status(p.Status.String()), "")
			if !ok {
				return Ticket{}, nil, ErrInvalidStatus
			}
			existing.Status = status
		}
		if p.Assignee.Set {
			existing.Assignee = normalizeAssignee(p.Assignee.Value)
		}
		if !p.Title.Set && !p.Status.Set && !p.Assignee.Set {
			return Ticket{}, nil, ErrEmptyPatch
		}
		return s.prepareUpdate(actor, existing, previous)
	})
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
	return s.repo.AddComment(ctx, actor.ID, ticketID, actor, s.now, Comment{Body: text, CreatedBy: actor})
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
	return s.repo.UpdateComment(ctx, actor.ID, ticketID, parsedComment, actor, s.now, text)
}

func (s *Service) DeleteComment(ctx context.Context, actor Actor, id string, commentID string) (Ticket, error) {
	ticketID, parsedComment, err := parseCommentIDs(id, commentID)
	if err != nil {
		return Ticket{}, err
	}
	return s.repo.DeleteComment(ctx, actor.ID, ticketID, parsedComment, actor, s.now)
}

func (s *Service) prepareUpdate(actor Actor, ticket Ticket, previous Status) (Ticket, []HistoryItem, error) {
	now := s.now()
	ticket.UpdatedBy, ticket.UpdatedAt = actor, now
	var history []HistoryItem
	if previous != ticket.Status {
		operationID, err := s.generateOperationID()
		if err != nil {
			return Ticket{}, nil, err
		}
		history = buildStatusHistory(operationID, previous, ticket.Status, actor, now)
	}
	return ticket, history, nil
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
	// page・pageSize は int の上限まで受け付けるため、乗算せずに範囲を判定する。
	if len(items) == 0 || page-1 > (len(items)-1)/pageSize {
		return []Ticket{}
	}
	start := (page - 1) * pageSize
	end := len(items)
	if pageSize < end-start {
		end = start + pageSize
	}
	return items[start:end]
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
