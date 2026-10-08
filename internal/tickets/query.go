package tickets

import (
	"net/url"
	"strconv"
	"strings"
)

const (
	defaultPage     = 1
	defaultPageSize = 10
)

// NormalizeListQuery は不正な値を既定値へ丸める。一覧 API は query の不正で 400 を返さない。
func NormalizeListQuery(values url.Values) ListQuery {
	return ListQuery{
		Q:         strings.TrimSpace(values.Get("q")),
		Status:    normalizeStatusFilter(values.Get("status")),
		SortBy:    normalizeSortBy(values.Get("sortBy")),
		SortOrder: normalizeSortOrder(values.Get("sortOrder")),
		Page:      parsePositiveInt(values.Get("page"), defaultPage),
		PageSize:  parsePositiveInt(values.Get("pageSize"), defaultPageSize),
	}
}

func normalizeStatusFilter(raw string) Status {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(StatusOpen):
		return StatusOpen
	case string(StatusClosed):
		return StatusClosed
	default:
		return StatusAll
	}
}

func normalizeSortBy(raw string) SortBy {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(SortByUpdatedAt):
		return SortByUpdatedAt
	case string(SortByCreatedAt):
		return SortByCreatedAt
	default:
		return SortByID
	}
}

func normalizeSortOrder(raw string) SortOrder {
	if strings.ToLower(strings.TrimSpace(raw)) == string(SortOrderDsc) {
		return SortOrderDsc
	}
	return SortOrderAsc
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
