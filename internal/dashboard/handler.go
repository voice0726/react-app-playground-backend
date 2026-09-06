package dashboard

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/voice0726/react-app-playground-backend/internal/shared/dberror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/orgcontext"
	"gorm.io/gorm"
)

type Handler struct{ db *gorm.DB }

type InvoiceSummary struct {
	DraftCount        int64 `json:"draftCount"`
	OutstandingAmount int64 `json:"outstandingAmount"`
	OverdueCount      int64 `json:"overdueCount"`
	OverdueAmount     int64 `json:"overdueAmount"`
}

type Response struct {
	Customers      int64          `json:"customers"`
	Deals          int64          `json:"deals"`
	Invoices       int64          `json:"invoices"`
	Tasks          int64          `json:"tasks"`
	InvoiceSummary InvoiceSummary `json:"invoiceSummary"`
}

func NewHandler(db *gorm.DB) *Handler { return &Handler{db: db} }

func (h *Handler) Dashboard(c *echo.Context) error {
	scope, err := orgcontext.From(c.Request().Context())
	if err != nil {
		return httperror.New(httperror.Unauthenticated, "unauthenticated", "Authentication and organization are required.", err)
	}
	today, err := parseToday(c.QueryParam("today"))
	if err != nil {
		return err
	}
	var response Response
	err = h.db.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		for _, item := range []struct {
			table string
			out   *int64
		}{
			{table: "customers", out: &response.Customers},
			{table: "deals", out: &response.Deals},
			{table: "invoices", out: &response.Invoices},
			{table: "tasks", out: &response.Tasks},
		} {
			if err := tx.Table(item.table).Where("organization_id = ?", scope.OrganizationID).Count(item.out).Error; err != nil {
				return dberror.Classify(err)
			}
		}
		row := tx.Table("invoices").Where("organization_id = ?", scope.OrganizationID).Select(`
			COUNT(*) FILTER (WHERE status = 'draft') AS draft_count,
			COALESCE(SUM(amount) FILTER (WHERE status = 'sent'), 0) AS outstanding_amount,
			COUNT(*) FILTER (WHERE status = 'sent' AND due_date < ?) AS overdue_count,
			COALESCE(SUM(amount) FILTER (WHERE status = 'sent' AND due_date < ?), 0) AS overdue_amount`, today, today).Row()
		return row.Scan(&response.InvoiceSummary.DraftCount, &response.InvoiceSummary.OutstandingAmount, &response.InvoiceSummary.OverdueCount, &response.InvoiceSummary.OverdueAmount)
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		if _, ok := err.(*httperror.Error); ok {
			return err
		}
		return dberror.Classify(err)
	}
	return c.JSON(http.StatusOK, response)
}

func parseToday(raw string) (string, error) {
	if raw == "" {
		return time.Now().UTC().Format("2006-01-02"), nil
	}
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return "", httperror.New(httperror.Validation, "dashboard_query_validation", "Invalid today date.", nil)
	}
	return value.Format("2006-01-02"), nil
}

func (h *Handler) Register(g *echo.Group) { h.RegisterScoped(g) }

func (h *Handler) RegisterScoped(g *echo.Group) { g.GET("/dashboard", h.Dashboard) }
