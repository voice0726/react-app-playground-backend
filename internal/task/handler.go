package task

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httpjson"
	"github.com/voice0726/react-app-playground-backend/internal/shared/requestuser"
	"net/http"
)

type Handler struct{ s *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }
func (h *Handler) user(c *echo.Context) (string, error) {
	u, ok := c.Get("authenticated_user").(requestuser.AuthenticatedUser)
	if !ok || u.ID == "" {
		return "", httperror.New(httperror.Unauthenticated, "unauthenticated", "Authentication required.", nil)
	}
	return u.ID, nil
}
func (h *Handler) List(c *echo.Context) error {
	u, e := h.user(c)
	if e != nil {
		return e
	}
	v, e := h.s.List(c.Request().Context(), u)
	if e != nil {
		return e
	}
	o := make([]Response, 0, len(v))
	for _, x := range v {
		o = append(o, response(x))
	}
	return c.JSON(http.StatusOK, o)
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
	v, e := h.s.Create(c.Request().Context(), u, r)
	if e != nil {
		return e
	}
	return c.JSON(http.StatusCreated, response(v))
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
	v, e := h.s.Update(c.Request().Context(), u, id, r)
	if e != nil {
		return e
	}
	return c.JSON(http.StatusOK, response(v))
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
	if e = h.s.Delete(c.Request().Context(), u, id); e != nil {
		return e
	}
	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}
func (h *Handler) Register(g *echo.Group) {
	g.GET("/tasks", h.List)
	g.POST("/tasks", h.Create)
	g.PATCH("/tasks/:id", h.Update)
	g.DELETE("/tasks/:id", h.Delete)
}
