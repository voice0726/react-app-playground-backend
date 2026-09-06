package invoice

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httpjson"
	"github.com/voice0726/react-app-playground-backend/internal/shared/orgcontext"
	"github.com/voice0726/react-app-playground-backend/internal/shared/pagination"
)

type Handler struct{ s *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }
func (h *Handler) scope(c *echo.Context) (orgcontext.Scope, error) {
	scope, err := orgcontext.From(c.Request().Context())
	if err != nil {
		return orgcontext.Scope{}, httperror.New(httperror.Unauthenticated, "unauthenticated", "Authentication and organization are required.", err)
	}
	return scope, nil
}
func (h *Handler) List(c *echo.Context) error {
	scope, e := h.scope(c)
	if e != nil {
		return e
	}
	query, e := pagination.Parse(c.QueryParams(), "createdAt", invoiceSorts)
	if e != nil {
		return httperror.New(httperror.Validation, "pagination_validation", "Invalid list query.", e)
	}
	status := c.QueryParam("status")
	if status != "" && status != "draft" && status != "sent" && status != "paid" && status != "cancelled" {
		return httperror.New(httperror.Validation, "invoice_query_validation", "Invalid invoice filter.", nil)
	}
	customerID, e := parseCustomerQuery(c)
	if e != nil {
		return e
	}
	overdue, e := parseOverdue(c)
	if e != nil {
		return e
	}
	today, e := parseToday(c)
	if e != nil {
		return e
	}
	v, total, totalAmount, e := h.s.List(c.Request().Context(), scope, ListOptions{Query: query, Status: status, CustomerID: customerID, Overdue: overdue, Today: today})
	if e != nil {
		return e
	}
	o := make([]Response, 0, len(v))
	for _, x := range v {
		o = append(o, response(x))
	}
	if query.Paginated {
		return c.JSON(http.StatusOK, map[string]interface{}{"items": o, "total": total, "page": query.Page, "pageSize": query.PageSize, "totalAmount": totalAmount})
	}
	return c.JSON(http.StatusOK, o)
}
func (h *Handler) Find(c *echo.Context) error {
	scope, e := h.scope(c)
	if e != nil {
		return e
	}
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		return httperror.New(httperror.Validation, "invalid_id", "Invalid ID.", nil)
	}
	v, e := h.s.Find(c.Request().Context(), scope, id)
	if e != nil {
		return e
	}
	return c.JSON(http.StatusOK, response(v))
}
func (h *Handler) Create(c *echo.Context) error {
	scope, e := h.scope(c)
	if e != nil {
		return e
	}
	var r CreateRequest
	if httpjson.Decode(c.Request(), &r) != nil {
		return httperror.New(httperror.Validation, "invalid_json", "Invalid JSON request.", nil)
	}
	v, e := h.s.Create(c.Request().Context(), scope, r)
	if e != nil {
		if _, ok := e.(*httperror.Error); ok {
			return e
		}
		return httperror.New(httperror.Validation, "invoice_validation", "Invalid invoice.", e)
	}
	return c.JSON(http.StatusCreated, response(v))
}
func (h *Handler) Update(c *echo.Context) error {
	scope, e := h.scope(c)
	if e != nil {
		return e
	}
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		return httperror.New(httperror.Validation, "invalid_id", "Invalid ID.", nil)
	}
	var r UpdateRequest
	if httpjson.Decode(c.Request(), &r) != nil {
		return httperror.New(httperror.Validation, "invalid_json", "Invalid JSON request.", nil)
	}
	v, e := h.s.Update(c.Request().Context(), scope, id, r)
	if e != nil {
		if errors.Is(e, ErrTransition) {
			return httperror.New(httperror.Conflict, "invoice_status_conflict", "Invoice status transition is not allowed.", e)
		}
		return e
	}
	return c.JSON(http.StatusOK, response(v))
}
func (h *Handler) Register(g *echo.Group) {
	h.RegisterScoped(g)
}

func (h *Handler) RegisterScoped(g *echo.Group) {
	g.GET("/invoices/summary", h.Summary)
	g.GET("/invoices", h.List)
	g.POST("/invoices", h.Create)
	g.GET("/invoices/:id", h.Find)
	g.PATCH("/invoices/:id", h.Update)
}

func (h *Handler) Summary(c *echo.Context) error {
	scope, err := h.scope(c)
	if err != nil {
		return err
	}
	today, err := parseToday(c)
	if err != nil {
		return err
	}
	customerID, err := parseCustomerQuery(c)
	if err != nil {
		return err
	}
	value, err := h.s.Summary(c.Request().Context(), scope, today, customerID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, value)
}

func parseCustomerQuery(c *echo.Context) (*uuid.UUID, error) {
	raw, ok := c.QueryParams()["customerId"]
	if !ok {
		return nil, nil
	}
	if len(raw) != 1 {
		return nil, httperror.New(httperror.Validation, "invoice_query_validation", "Invalid customer filter.", nil)
	}
	value, err := uuid.Parse(raw[0])
	if err != nil {
		return nil, httperror.New(httperror.Validation, "invoice_query_validation", "Invalid customer filter.", nil)
	}
	return &value, nil
}

func parseOverdue(c *echo.Context) (bool, error) {
	raw, ok := c.QueryParams()["overdue"]
	if !ok {
		return false, nil
	}
	if len(raw) != 1 || (strings.ToLower(raw[0]) != "true" && strings.ToLower(raw[0]) != "false") {
		return false, httperror.New(httperror.Validation, "invoice_query_validation", "Invalid overdue filter.", nil)
	}
	return strings.ToLower(raw[0]) == "true", nil
}

func parseToday(c *echo.Context) (time.Time, error) {
	raw := c.QueryParam("today")
	if raw == "" {
		return time.Now().UTC(), nil
	}
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, httperror.New(httperror.Validation, "invoice_query_validation", "Invalid today date.", nil)
	}
	return value, nil
}
