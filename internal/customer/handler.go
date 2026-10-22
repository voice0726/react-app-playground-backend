package customer

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httpjson"
	"github.com/voice0726/react-app-playground-backend/internal/shared/orgcontext"
	"github.com/voice0726/react-app-playground-backend/internal/shared/pagination"
)

type Handler struct{ service *Service }

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
	query, e := pagination.Parse(c.QueryParams(), "createdAt", customerSorts)
	if e != nil {
		return httperror.New(httperror.Validation, "pagination_validation", "Invalid list query.", e)
	}
	status := c.QueryParam("status")
	if status != "" && status != "active" && status != "inactive" {
		return httperror.New(httperror.Validation, "customer_query_validation", "Invalid customer filter.", nil)
	}
	v, total, e := h.service.List(c.Request().Context(), scope, ListOptions{Query: query, Status: status})
	if e != nil {
		return e
	}
	out := make([]Response, 0, len(v))
	for _, x := range v {
		out = append(out, toResponse(x))
	}
	if query.Paginated {
		return c.JSON(http.StatusOK, map[string]interface{}{"items": out, "total": total, "page": query.Page, "pageSize": query.PageSize})
	}
	return c.JSON(http.StatusOK, out)
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
	v, e := h.service.Find(c.Request().Context(), scope, id)
	if e != nil {
		return e
	}
	return c.JSON(http.StatusOK, toResponse(v))
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
	v, e := h.service.Create(c.Request().Context(), scope, r)
	if e != nil {
		return e
	}
	return c.JSON(http.StatusCreated, toResponse(v))
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
	v, e := h.service.Update(c.Request().Context(), scope, id, r)
	if e != nil {
		return e
	}
	return c.JSON(http.StatusOK, toResponse(v))
}
func (h *Handler) Delete(c *echo.Context) error {
	scope, e := h.scope(c)
	if e != nil {
		return e
	}
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		return httperror.New(httperror.Validation, "invalid_id", "Invalid ID.", nil)
	}
	if e = h.service.Delete(c.Request().Context(), scope, id); e != nil {
		return e
	}
	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}
func (h *Handler) Register(g *echo.Group) {
	h.RegisterScoped(g)
}

func (h *Handler) RegisterScoped(g *echo.Group) {
	g.GET("/customers", h.List)
	g.POST("/customers", h.Create)
	g.GET("/customers/:id", h.Find)
	g.PATCH("/customers/:id", h.Update)
	g.DELETE("/customers/:id", h.Delete)
}
