package invoice

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/dberror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/pagination"
	"gorm.io/gorm"
	"time"

	"gorm.io/gorm/clause"
)

type GORMRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db} }
func (r *GORMRepository) List(c context.Context, organizationID uuid.UUID, options ListOptions) ([]Invoice, int64, int64, error) {
	var v []Invoice
	var total, totalAmount int64
	e := r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		query := invoiceQuery(tx, organizationID, options)
		if err := query.Model(&Invoice{}).Count(&total).Error; err != nil {
			return dberror.Classify(err)
		}
		if err := query.Session(&gorm.Session{}).Select("COALESCE(SUM(amount), 0)").Scan(&totalAmount).Error; err != nil {
			return dberror.Classify(err)
		}
		query = invoiceQuery(tx, organizationID, options).
			Order(pagination.SortExpression(options.Query, invoiceSorts)).Order(invoiceIDOrder(options.Query))
		if options.Query.Paginated {
			offset, err := options.Query.Offset()
			if err != nil {
				return httperror.New(httperror.Validation, "pagination_validation", "Invalid pagination.", err)
			}
			query = query.Offset(offset).Limit(options.Query.PageSize)
		}
		if err := query.Find(&v).Error; err != nil {
			return dberror.Classify(err)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return nil, 0, 0, e
	}
	return v, total, totalAmount, nil
}
func (r *GORMRepository) Find(c context.Context, organizationID, id uuid.UUID) (Invoice, error) {
	var v Invoice
	e := r.db.WithContext(c).Where("organization_id = ? AND id = ?", organizationID, id).First(&v).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return v, httperror.New(httperror.NotFound, "invoice_not_found", "Invoice not found.", nil)
	}
	if e != nil {
		return v, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) Create(c context.Context, organizationID uuid.UUID, userID string, v Invoice) (Invoice, error) {
	v.ID = uuid.New()
	v.OrganizationID = organizationID
	v.OwnerUserID = userID
	e := r.db.WithContext(c).Create(&v).Error
	if e != nil {
		return v, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) UpdateLocked(c context.Context, organizationID, id uuid.UUID, apply func(Invoice) (Invoice, error)) (Invoice, error) {
	var out Invoice
	e := r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		if e := tx.Where("organization_id = ? AND id = ?", organizationID, id).Clauses(clause.Locking{Strength: "UPDATE"}).First(&out).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return httperror.New(httperror.NotFound, "invoice_not_found", "Invoice not found.", nil)
			}
			return dberror.Classify(e)
		}
		next, e := apply(out)
		if e != nil {
			return e
		}
		next.OrganizationID = organizationID
		next.ID = id
		next.UpdatedAt = out.UpdatedAt
		if e = tx.Save(&next).Error; e != nil {
			return dberror.Classify(e)
		}
		out = next
		return nil
	})
	if e != nil {
		var httpErr *httperror.Error
		if errors.As(e, &httpErr) || errors.Is(e, ErrTransition) {
			return out, e
		}
		return out, dberror.Classify(e)
	}
	return out, nil
}

func (r *GORMRepository) UpdateStatuses(c context.Context, organizationID uuid.UUID, ids []uuid.UUID, from, to string) ([]Invoice, error) {
	var rows []Invoice
	e := r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		// 一括更新同士がロック待ちで循環しないよう、ID 順に行ロックを取る。
		if e := tx.Where("organization_id = ? AND id IN ?", organizationID, ids).Order("id").Clauses(clause.Locking{Strength: "UPDATE"}).Find(&rows).Error; e != nil {
			return dberror.Classify(e)
		}
		byID := make(map[uuid.UUID]Invoice, len(rows))
		for _, row := range rows {
			byID[row.ID] = row
		}
		missing := []string{}
		conflicted := []string{}
		for _, id := range ids {
			row, ok := byID[id]
			if !ok {
				missing = append(missing, id.String())
			} else if row.Status != from {
				conflicted = append(conflicted, id.String())
			}
		}
		if len(missing) > 0 {
			return httperror.New(httperror.NotFound, "invoice_not_found", "Some invoices were not found.", nil).WithDetails(map[string]interface{}{"invoiceIds": missing})
		}
		if len(conflicted) > 0 {
			return httperror.New(httperror.Conflict, "invoice_status_conflict", "Some invoices are no longer in the expected status.", nil).WithDetails(map[string]interface{}{"invoiceIds": conflicted})
		}
		result := tx.Model(&Invoice{}).
			Where("organization_id = ? AND id IN ? AND status = ?", organizationID, ids, from).
			Update("status", to)
		if result.Error != nil {
			return dberror.Classify(result.Error)
		}
		if result.RowsAffected != int64(len(ids)) {
			// Rows may change between validation and update on databases without row locks.
			conflicted := make([]string, 0, len(ids))
			for _, id := range ids {
				conflicted = append(conflicted, id.String())
			}
			return httperror.New(httperror.Conflict, "invoice_status_conflict", "Some invoices are no longer in the expected status.", nil).
				WithDetails(map[string]interface{}{"invoiceIds": conflicted})
		}
		rows = make([]Invoice, 0, len(ids))
		for _, id := range ids {
			row := byID[id]
			row.Status = to
			rows = append(rows, row)
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	return rows, nil
}

func (r *GORMRepository) Summary(ctx context.Context, organizationID uuid.UUID, today time.Time, customerID *uuid.UUID) (Summary, error) {
	var summary Summary
	query := r.db.WithContext(ctx).Model(&Invoice{}).Where("organization_id = ?", organizationID)
	if customerID != nil {
		query = query.Where("customer_id = ?", *customerID)
	}
	todayDate := today.Format("2006-01-02")
	row := query.Select(`
		COUNT(*) FILTER (WHERE status = 'draft') AS draft_count,
		COALESCE(SUM(amount) FILTER (WHERE status = 'sent'), 0) AS outstanding_amount,
		COUNT(*) FILTER (WHERE status = 'sent' AND due_date < ?) AS overdue_count,
		COALESCE(SUM(amount) FILTER (WHERE status = 'sent' AND due_date < ?), 0) AS overdue_amount`, todayDate, todayDate).Row()
	if err := row.Scan(&summary.DraftCount, &summary.OutstandingAmount, &summary.OverdueCount, &summary.OverdueAmount); err != nil {
		return Summary{}, dberror.Classify(err)
	}
	return summary, nil
}

var invoiceSorts = map[string]string{
	"amount":     "amount",
	"createdAt":  "created_at",
	"customerId": "customer_id",
	"dueDate":    "due_date",
	"id":         "id",
	"issueDate":  "issue_date",
	"number":     "number",
	"status":     "status",
	"updatedAt":  "updated_at",
}

func invoiceQuery(tx *gorm.DB, organizationID uuid.UUID, options ListOptions) *gorm.DB {
	query := tx.Where("organization_id = ?", organizationID)
	if options.Query.Q != "" {
		query = query.Where("number ILIKE ? ESCAPE '\\'", pagination.LikePattern(options.Query.Q))
	}
	if options.Status != "" {
		query = query.Where("status = ?", options.Status)
	}
	if options.CustomerID != nil {
		query = query.Where("customer_id = ?", *options.CustomerID)
	}
	if options.Overdue {
		query = query.Where("status = ? AND due_date < ?", "sent", options.Today.Format("2006-01-02"))
	}
	return query
}

func invoiceIDOrder(query pagination.Query) string {
	if query.Descending {
		return "id DESC"
	}
	return "id ASC"
}
