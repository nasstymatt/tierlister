package db

import (
	"net/http"
	"strconv"
)

type Page[T any] struct {
	Items      []T
	Page       int
	PerPage    int
	TotalItems int
	TotalPages int
	HasNext    bool
	HasPrev    bool
	Offset     int
}

func Paginate[T any](items []T, page, perPage, total int) Page[T] {
	if perPage <= 0 {
		perPage = 20
	}
	if page <= 0 {
		page = 1
	}
	totalPages := (total + perPage - 1) / perPage
	offset := (page - 1) * perPage

	return Page[T]{
		Items:      items,
		Page:       page,
		PerPage:    perPage,
		TotalItems: total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
		Offset:     offset,
	}
}

func PaginationParams(r *http.Request) (page, perPage, offset int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ = strconv.Atoi(r.URL.Query().Get("per_page"))
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	offset = (page - 1) * perPage
	return
}
