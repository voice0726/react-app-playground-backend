package organization

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httpjson"
	"github.com/voice0726/react-app-playground-backend/internal/shared/requestuser"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) user(c *echo.Context) (requestuser.AuthenticatedUser, error) {
	user, err := requestuser.From(c.Request().Context())
	if err != nil {
		return requestuser.AuthenticatedUser{}, httperror.New(httperror.Unauthenticated, "unauthenticated", "Authentication required.", err)
	}
	return user, nil
}

func parseOrganizationID(c *echo.Context) (uuid.UUID, error) {
	organizationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, httperror.New(httperror.Validation, "invalid_organization_id", "Invalid organization ID.", nil)
	}
	return organizationID, nil
}

func (h *Handler) List(c *echo.Context) error {
	user, err := h.user(c)
	if err != nil {
		return err
	}
	organizations, err := h.service.List(c.Request().Context(), user.ID)
	if err != nil {
		return err
	}
	response := make([]OrganizationResponse, 0, len(organizations))
	for _, organization := range organizations {
		response = append(response, organizationResponse(organization))
	}
	return c.JSON(http.StatusOK, response)
}

func (h *Handler) Create(c *echo.Context) error {
	user, err := h.user(c)
	if err != nil {
		return err
	}
	var request CreateRequest
	if err := httpjson.Decode(c.Request(), &request); err != nil {
		return httperror.New(httperror.Validation, "invalid_json", "Invalid JSON request.", err)
	}
	organization, err := h.service.Create(c.Request().Context(), user.ID, request.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, organizationResponse(organization))
}

func (h *Handler) ListMembers(c *echo.Context) error {
	user, err := h.user(c)
	if err != nil {
		return err
	}
	organizationID, err := parseOrganizationID(c)
	if err != nil {
		return err
	}
	members, err := h.service.ListMembers(c.Request().Context(), user.ID, organizationID)
	if err != nil {
		return err
	}
	response := make([]MemberResponse, 0, len(members))
	for _, member := range members {
		response = append(response, memberResponse(member))
	}
	return c.JSON(http.StatusOK, response)
}

func (h *Handler) AddMember(c *echo.Context) error {
	user, err := h.user(c)
	if err != nil {
		return err
	}
	organizationID, err := parseOrganizationID(c)
	if err != nil {
		return err
	}
	var request MemberRequest
	if err := httpjson.Decode(c.Request(), &request); err != nil {
		return httperror.New(httperror.Validation, "invalid_json", "Invalid JSON request.", err)
	}
	member, err := h.service.AddMember(c.Request().Context(), user.ID, organizationID, request)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, memberResponse(member))
}

func (h *Handler) UpdateMember(c *echo.Context) error {
	user, err := h.user(c)
	if err != nil {
		return err
	}
	organizationID, err := parseOrganizationID(c)
	if err != nil {
		return err
	}
	var request struct {
		Role string `json:"role"`
	}
	if err := httpjson.Decode(c.Request(), &request); err != nil {
		return httperror.New(httperror.Validation, "invalid_json", "Invalid JSON request.", err)
	}
	member, err := h.service.UpdateMemberRole(c.Request().Context(), user.ID, organizationID, c.Param("userId"), request.Role)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, memberResponse(member))
}

func (h *Handler) RemoveMember(c *echo.Context) error {
	user, err := h.user(c)
	if err != nil {
		return err
	}
	organizationID, err := parseOrganizationID(c)
	if err != nil {
		return err
	}
	if err := h.service.RemoveMember(c.Request().Context(), user.ID, organizationID, c.Param("userId")); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

func (h *Handler) Register(g *echo.Group) {
	g.GET("/organizations", h.List)
	g.POST("/organizations", h.Create)
	g.GET("/organizations/:id/members", h.ListMembers)
	g.POST("/organizations/:id/members", h.AddMember)
	g.PATCH("/organizations/:id/members/:userId", h.UpdateMember)
	g.DELETE("/organizations/:id/members/:userId", h.RemoveMember)
}
