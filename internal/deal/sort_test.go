package deal

import (
	"net/url"
	"testing"

	"github.com/voice0726/react-app-playground-backend/internal/shared/pagination"
)

func TestCustomerSortUsesCustomerIDColumn(t *testing.T) {
	for _, order := range []string{"asc", "desc"} {
		t.Run(order, func(t *testing.T) {
			query, err := pagination.Parse(url.Values{"sort": {"customerId"}, "order": {order}}, "createdAt", dealSorts)
			if err != nil {
				t.Fatalf("customer column sort was rejected: %v", err)
			}
			want := "customer_id ASC"
			if order == "desc" {
				want = "customer_id DESC"
			}
			if actual := pagination.SortExpression(query, dealSorts); actual != want {
				t.Fatalf("sort expression=%q, want %q", actual, want)
			}
		})
	}
}
