package internal

import (
	"net/url"
	"strconv"
	"time"
)

// URL filtering options
type SiftURLOption func(v url.Values)

func WithAscending(asc bool) SiftURLOption {
	return func(v url.Values) {
		v.Set("ascending", strconv.FormatBool(asc))
	}
}

func WithOrder(field string) SiftURLOption {
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

func WithQuery(searchTerm string) SiftURLOption {
	return func(v url.Values) {
		if searchTerm != "" {
			v.Set("q", searchTerm)
		}
	}
}

func WithTagID(id int) SiftURLOption {
	return func(v url.Values) {
		v.Set("tag_id", strconv.Itoa(id))
	}
}

func WithRelatedTags(related bool) SiftURLOption {
	return func(v url.Values) {
		v.Set("related_tags", strconv.FormatBool(related))
	}
}

func WithSlug(slug string) SiftURLOption {
	return func(v url.Values) {
		if slug != "" {
			v.Set("slug", slug)
		}
	}
}

func NewEventsURL(opts ...SiftURLOption) string {
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
type SiftAdvancedOption func(c *ClientEvent) bool

func ExpiresBetween(min, max time.Duration) SiftAdvancedOption {
	return func(c *ClientEvent) bool {
		if remaining := time.Until(time.Time(c.EndDate)); min < remaining && remaining < max {
			return true
		}

		return false
	}
}

func NewSiftAdvancedOption(opts ...SiftAdvancedOption) SiftAdvancedOption {
	
	return func(c *ClientEvent) bool {
		for _, opt := range opts {
			if !opt(c) {
				return false
			}
		}

		return true
	}
}