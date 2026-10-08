package tickets

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httpjson"
	"github.com/voice0726/react-app-playground-backend/internal/shared/requestuser"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }

func (h *Handler) Register(g *echo.Group) {
	g.GET("/tickets", h.List)
	g.GET("/tickets/:id", h.Find)
	g.POST("/tickets", h.Create)
	g.PUT("/tickets/:id", h.Replace)
	g.PATCH("/tickets/:id", h.Patch)
	g.DELETE("/tickets/:id", h.Delete)
	g.POST("/tickets/:id/comments", h.AddComment)
	g.PUT("/tickets/:id/comments/:commentId", h.UpdateComment)
	g.DELETE("/tickets/:id/comments/:commentId", h.DeleteComment)
}

func (h *Handler) actor(c *echo.Context) (Actor, error) {
	u, ok := c.Get("authenticated_user").(requestuser.AuthenticatedUser)
	if !ok || u.ID == "" {
		return Actor{}, httperror.New(httperror.Unauthenticated, "unauthenticated", "Authentication required.", nil)
	}
	return Actor{ID: u.ID, Name: u.Name, Email: u.Email}, nil
}

func decode(c *echo.Context, dst any) error {
	if httpjson.Decode(c.Request(), dst) != nil {
		return httperror.New(httperror.Validation, "invalid_json", "Invalid JSON request.", nil)
	}
	return nil
}

func (h *Handler) List(c *echo.Context) error {
	a, err := h.actor(c)
	if err != nil {
		return err
	}
	result, err := h.service.List(c.Request().Context(), a.ID, NormalizeListQuery(c.QueryParams()))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, listResponse(result))
}

func (h *Handler) Find(c *echo.Context) error {
	a, err := h.actor(c)
	if err != nil {
		return err
	}
	t, err := h.service.Find(c.Request().Context(), a.ID, c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response(t))
}

func (h *Handler) Create(c *echo.Context) error {
	a, err := h.actor(c)
	if err != nil {
		return err
	}
	var r CreateRequest
	if err := decode(c, &r); err != nil {
		return err
	}
	var status Status
	if r.Status != nil {
		status = Status(*r.Status)
	}
	t, err := h.service.Create(c.Request().Context(), a, CreateParams{Title: r.Title, Status: status, Assignee: r.Assignee})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, response(t))
}

func (h *Handler) Replace(c *echo.Context) error {
	a, err := h.actor(c)
	if err != nil {
		return err
	}
	var r ReplaceRequest
	if err := decode(c, &r); err != nil {
		return err
	}
	t, err := h.service.Replace(c.Request().Context(), a, c.Param("id"), ReplaceParams{Title: r.Title, Status: Status(r.Status), Assignee: r.Assignee})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response(t))
}

func (h *Handler) Patch(c *echo.Context) error {
	a, err := h.actor(c)
	if err != nil {
		return err
	}
	var r PatchRequest
	if err := decode(c, &r); err != nil {
		return err
	}
	t, err := h.service.Patch(c.Request().Context(), a, c.Param("id"), PatchParams(r))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response(t))
}

func (h *Handler) Delete(c *echo.Context) error {
	a, err := h.actor(c)
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Request().Context(), a.ID, c.Param("id")); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) AddComment(c *echo.Context) error {
	a, err := h.actor(c)
	if err != nil {
		return err
	}
	var r CommentRequest
	if err := decode(c, &r); err != nil {
		return err
	}
	t, err := h.service.AddComment(c.Request().Context(), a, c.Param("id"), r.Body)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, response(t))
}

func (h *Handler) UpdateComment(c *echo.Context) error {
	a, err := h.actor(c)
	if err != nil {
		return err
	}
	var r CommentRequest
	if err := decode(c, &r); err != nil {
		return err
	}
	t, err := h.service.UpdateComment(c.Request().Context(), a, c.Param("id"), c.Param("commentId"), r.Body)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response(t))
}

func (h *Handler) DeleteComment(c *echo.Context) error {
	a, err := h.actor(c)
	if err != nil {
		return err
	}
	t, err := h.service.DeleteComment(c.Request().Context(), a, c.Param("id"), c.Param("commentId"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response(t))
}
