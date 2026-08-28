package customer

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httpjson"
	"github.com/voice0726/react-app-playground-backend/internal/shared/requestuser"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }
func (h *Handler) user(c *echo.Context) (requestuser.AuthenticatedUser, error) {
	v, ok := c.Get("authenticated_user").(requestuser.AuthenticatedUser)
	if !ok || v.ID == "" {
		return v, httperror.New(httperror.Unauthenticated, "unauthenticated", "Authentication required.", nil)
	}
	return v, nil
}
func (h *Handler) List(c *echo.Context) error {
	u, e := h.user(c)
	if e != nil {
		return e
	}
	v, e := h.service.List(c.Request().Context(), u.ID)
	if e != nil {
		return e
	}
	out := make([]Response, 0, len(v))
	for _, x := range v {
		out = append(out, toResponse(x))
	}
	return c.JSON(http.StatusOK, out)
}
func (h *Handler) Find(c *echo.Context) error {
	u, e := h.user(c)
	if e != nil {
		return e
	}
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		return httperror.New(httperror.Validation, "invalid_id", "Invalid ID.", nil)
	}
	v, e := h.service.Find(c.Request().Context(), u.ID, id)
	if e != nil {
		return e
	}
	return c.JSON(http.StatusOK, toResponse(v))
}
func (h *Handler) Create(c *echo.Context) error {
	u, e := h.user(c)
	if e != nil {
		return e
	}
	var r CreateRequest
	if httpjson.Decode(c.Request(), &r) != nil {
		return httperror.New(httperror.Validation, "invalid_json", "Invalid JSON request.", nil)
	}
	v, e := h.service.Create(c.Request().Context(), u.ID, r)
	if e != nil {
		return e
	}
	return c.JSON(http.StatusCreated, toResponse(v))
}
func (h *Handler) Update(c *echo.Context) error {
	u, e := h.user(c)
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
	v, e := h.service.Update(c.Request().Context(), u.ID, id, r)
	if e != nil {
		return e
	}
	return c.JSON(http.StatusOK, toResponse(v))
}
func (h *Handler) Delete(c *echo.Context) error {
	u, e := h.user(c)
	if e != nil {
		return e
	}
	id, e := uuid.Parse(c.Param("id"))
	if e != nil {
		return httperror.New(httperror.Validation, "invalid_id", "Invalid ID.", nil)
	}
	if e = h.service.Delete(c.Request().Context(), u.ID, id); e != nil {
		return e
	}
	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}
func (h *Handler) Register(g *echo.Group) {
	g.GET("/customers", h.List)
	g.POST("/customers", h.Create)
	g.GET("/customers/:id", h.Find)
	g.PATCH("/customers/:id", h.Update)
	g.DELETE("/customers/:id", h.Delete)
}
