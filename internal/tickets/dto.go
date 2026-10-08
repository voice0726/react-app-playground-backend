package tickets

import (
	"bytes"
	"encoding/json"
	"time"
)

// OptionalString は JSON の「省略」と「null」を区別する。キーが存在すると Set が true になる。
type OptionalString struct {
	Set   bool
	Value *string
}

func (o *OptionalString) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Value = nil
		return nil
	}
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

func (o OptionalString) String() string {
	if o.Value == nil {
		return ""
	}
	return *o.Value
}

type CreateRequest struct {
	Title    string  `json:"title"`
	Status   *string `json:"status"`
	Assignee *string `json:"assignee"`
}

type ReplaceRequest struct {
	Title    string  `json:"title"`
	Status   string  `json:"status"`
	Assignee *string `json:"assignee"`
}

type PatchRequest struct {
	Title    OptionalString `json:"title"`
	Status   OptionalString `json:"status"`
	Assignee OptionalString `json:"assignee"`
}

type CommentRequest struct {
	Body string `json:"body"`
}

type ListResponse struct {
	Items []SummaryResponse `json:"items"`
	Total int               `json:"total"`
}

type SummaryResponse struct {
	ID        uint64        `json:"id"`
	Title     string        `json:"title"`
	Status    string        `json:"status"`
	Assignee  *string       `json:"assignee,omitempty"`
	CreatedBy ActorResponse `json:"createdBy"`
	UpdatedBy ActorResponse `json:"updatedBy"`
	CreatedAt string        `json:"createdAt"`
	UpdatedAt string        `json:"updatedAt"`
}

type Response struct {
	ID        uint64           `json:"id"`
	Title     string           `json:"title"`
	Status    string           `json:"status"`
	Assignee  *string          `json:"assignee,omitempty"`
	CreatedBy ActorResponse    `json:"createdBy"`
	UpdatedBy ActorResponse    `json:"updatedBy"`
	CreatedAt string           `json:"createdAt"`
	UpdatedAt string           `json:"updatedAt"`
	History   HistoryResponse  `json:"history"`
	Comments  CommentsResponse `json:"comments"`
}

type ActorResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

type HistoryResponse struct {
	Items []HistoryItemResponse `json:"items"`
}

type HistoryItemResponse struct {
	OperationID string                  `json:"operationId"`
	Actor       ActorResponse           `json:"actor"`
	ChangedAt   string                  `json:"changedAt"`
	Changes     []HistoryChangeResponse `json:"changes"`
}

type HistoryChangeResponse struct {
	Field  string  `json:"field"`
	Before *string `json:"before"`
	After  *string `json:"after"`
}

type CommentsResponse struct {
	Items []CommentResponse `json:"items"`
}

type CommentResponse struct {
	ID        uint64        `json:"id"`
	Body      string        `json:"body"`
	CreatedBy ActorResponse `json:"createdBy"`
	CreatedAt string        `json:"createdAt"`
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func actorResponse(a Actor) ActorResponse {
	return ActorResponse{ID: a.ID, Email: a.Email, DisplayName: a.Name}
}

func summaryResponse(t Ticket) SummaryResponse {
	return SummaryResponse{
		ID: t.ID, Title: t.Title, Status: string(t.Status), Assignee: t.Assignee,
		CreatedBy: actorResponse(t.CreatedBy), UpdatedBy: actorResponse(t.UpdatedBy),
		CreatedAt: formatTime(t.CreatedAt), UpdatedAt: formatTime(t.UpdatedAt),
	}
}

func response(t Ticket) Response {
	history := make([]HistoryItemResponse, 0, len(t.History))
	for _, item := range t.History {
		changes := make([]HistoryChangeResponse, 0, len(item.Changes))
		for _, c := range item.Changes {
			changes = append(changes, HistoryChangeResponse{Field: string(c.Field), Before: c.Before, After: c.After})
		}
		history = append(history, HistoryItemResponse{
			OperationID: item.OperationID, Actor: actorResponse(item.Actor),
			ChangedAt: formatTime(item.ChangedAt), Changes: changes,
		})
	}
	comments := make([]CommentResponse, 0, len(t.Comments))
	for _, c := range t.Comments {
		comments = append(comments, CommentResponse{ID: c.ID, Body: c.Body, CreatedBy: actorResponse(c.CreatedBy), CreatedAt: formatTime(c.CreatedAt)})
	}
	s := summaryResponse(t)
	return Response{
		ID: s.ID, Title: s.Title, Status: s.Status, Assignee: s.Assignee,
		CreatedBy: s.CreatedBy, UpdatedBy: s.UpdatedBy, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
		History: HistoryResponse{Items: history}, Comments: CommentsResponse{Items: comments},
	}
}

func listResponse(r ListResult) ListResponse {
	items := make([]SummaryResponse, 0, len(r.Items))
	for _, t := range r.Items {
		items = append(items, summaryResponse(t))
	}
	return ListResponse{Items: items, Total: r.Total}
}
