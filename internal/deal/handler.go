package deal

import (
	"net/http"

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
	query, e := pagination.Parse(c.QueryParams(), "createdAt", dealSorts)
	if e != nil {
		return httperror.New(httperror.Validation, "pagination_validation", "Invalid list query.", e)
	}
	status := c.QueryParam("status")
	if status != "" && status != "open" && status != "won" && status != "lost" {
		return httperror.New(httperror.Validation, "deal_query_validation", "Invalid deal filter.", nil)
	}
	var customerID *uuid.UUID
	if raw, ok := c.QueryParams()["customerId"]; ok {
		if len(raw) != 1 {
			return httperror.New(httperror.Validation, "deal_query_validation", "Invalid customer filter.", nil)
		}
		parsed, parseErr := uuid.Parse(raw[0])
		if parseErr != nil {
			return httperror.New(httperror.Validation, "deal_query_validation", "Invalid customer filter.", nil)
		}
		customerID = &parsed
	}
	v, total, e := h.s.List(c.Request().Context(), scope, ListOptions{Query: query, Status: status, CustomerID: customerID})
	if e != nil {
		return e
	}
	o := make([]Response, 0, len(v))
	for _, x := range v {
		o = append(o, response(x))
	}
	if query.Paginated {
		return c.JSON(http.StatusOK, map[string]interface{}{"items": o, "total": total, "page": query.Page, "pageSize": query.PageSize})
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
		return httperror.New(httperror.Validation, "deal_validation", "Invalid deal.", e)
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
		return e
	}
	return c.JSON(http.StatusOK, response(v))
}
func (h *Handler) Register(g *echo.Group) {
	h.RegisterScoped(g)
}

func (h *Handler) RegisterScoped(g *echo.Group) {
	g.GET("/deals", h.List)
	g.POST("/deals", h.Create)
	g.GET("/deals/:id", h.Find)
	g.PATCH("/deals/:id", h.Update)
}
