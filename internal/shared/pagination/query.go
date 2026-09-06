package pagination

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
	MaxQueryLength  = 200
)

type Query struct {
	Page       int
	PageSize   int
	Paginated  bool
	Q          string
	Sort       string
	Descending bool
}

func (q Query) Offset() (int, error) {
	maxInt := int(^uint(0) >> 1)
	if q.Page < 1 || q.PageSize < 1 || q.Page-1 > maxInt/q.PageSize {
		return 0, errors.New("page is too large")
	}
	return (q.Page - 1) * q.PageSize, nil
}

func Parse(values url.Values, defaultSort string, allowedSort map[string]string) (Query, error) {
	_, pageProvided := values["page"]
	_, pageSizeProvided := values["pageSize"]
	query := Query{Page: DefaultPage, PageSize: DefaultPageSize, Paginated: pageProvided || pageSizeProvided, Sort: defaultSort}
	if pageProvided {
		page, err := parsePositive(values.Get("page"), "page", 0)
		if err != nil {
			return Query{}, err
		}
		query.Page = page
	}
	if pageSizeProvided {
		pageSize, err := parsePositive(values.Get("pageSize"), "pageSize", MaxPageSize)
		if err != nil {
			return Query{}, err
		}
		query.PageSize = pageSize
	}
	if raw := strings.TrimSpace(values.Get("q")); raw != "" {
		if utf8.RuneCountInString(raw) > MaxQueryLength {
			return Query{}, errors.New("q is too long")
		}
		query.Q = raw
	}
	if raw, ok := values["sort"]; ok {
		if len(raw) == 0 || raw[0] == "" {
			return Query{}, errors.New("sort is invalid")
		}
		query.Sort = raw[0]
	}
	if _, ok := allowedSort[query.Sort]; !ok {
		return Query{}, errors.New("sort is invalid")
	}
	if raw, ok := values["order"]; ok {
		if len(raw) == 0 || raw[0] == "" {
			return Query{}, errors.New("order is invalid")
		}
		switch strings.ToLower(raw[0]) {
		case "asc":
		case "desc":
			query.Descending = true
		default:
			return Query{}, errors.New("order is invalid")
		}
	}
	return query, nil
}

func SortExpression(query Query, allowedSort map[string]string) string {
	direction := "ASC"
	if query.Descending {
		direction = "DESC"
	}
	return allowedSort[query.Sort] + " " + direction
}

func LikePattern(value string) string {
	value = strings.ReplaceAll(value, `\`, `\`+`\`)
	value = strings.ReplaceAll(value, "%", `\%`)
	value = strings.ReplaceAll(value, "_", `\_`)
	return "%" + value + "%"
}

func parsePositive(value, name string, max int) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || (max > 0 && parsed > max) {
		return 0, errors.New(name + " is invalid")
	}
	return parsed, nil
}
