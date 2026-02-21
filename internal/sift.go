package internal

import (
	"net/url"
	"strconv"
)

// URL filtering options
type SiftOption func(v url.Values)

func WithAscending(asc bool) SiftOption {
	return func(v url.Values) {
		v.Set("ascending", strconv.FormatBool(asc))
	}
}

func WithOrder(field string) SiftOption {
	allowed := map[string]bool{
		"id":          true,
		"startDate":   true,
		"endDate":     true,
		"volume":      true,
		"volume24hr":  true,
		"liquidity":   true,
		"createdAt":   true,
		"closedTime":  true,
		"competitive": true,
	}

	return func(v url.Values) {
		if allowed[field] {
			v.Set("order", field)
		}
	}
}

func WithQuery(searchTerm string) SiftOption {
	return func(v url.Values) {
		if searchTerm != "" {
			v.Set("q", searchTerm)
		}
	}
}

func WithTagID(id int) SiftOption {
	return func(v url.Values) {
		v.Set("tag_id", strconv.Itoa(id))
	}
}

func WithRelatedTags(related bool) SiftOption {
	return func(v url.Values) {
		v.Set("related_tags", strconv.FormatBool(related))
	}
}

func WithSlug(slug string) SiftOption {
	return func(v url.Values) {
		if slug != "" {
			v.Set("slug", slug)
		}
	}
}

func NewEventsURL(opts ...SiftOption) string {
	u, _ := url.Parse("https://gamma-api.polymarket.com/events")

	q := u.Query()
	q.Set("active", "true")
	q.Set("closed", "false")

	for _, opt := range opts {
		opt(q)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// Advanced filtering options
