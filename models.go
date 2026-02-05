package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Polymarket Models
type PolyEvent struct {
	ID           int
	EventID      PolyInt      `json:"id"`
	Title        string       `json:"title"`
	Description  string       `json:"description"`
	StartDate    string       `json:"startDate"`
	CreationDate string       `json:"creationDate"`
	EndDate      string       `json:"endDate"`
	Active       bool         `json:"active"`
	Closed       bool         `json:"closed"`
	Archived     bool         `json:"archived"`
	Volume       PolyFloat    `json:"volume"`
	Category     string       `json:"category"`
	PublishedAt  string       `json:"published_at"`
	CreatedAt    string       `json:"createdAt"`
	UpdatedAt    string       `json:"updatedAt"`
	Markets      []PolyMarket `json:"markets"`
	Tags         []PolyTag    `json:"tags"`
}

type PolyMarket struct {
	ID            int
	MarketID      PolyInt           `json:"id"`
	Question      string            `json:"question"`
	EndDate       string            `json:"endDate"`
	StartDate     string            `json:"startDate"`
	Description   string            `json:"description"`
	Outcomes      PolyOutcomes      `json:"outcomes"`
	OutcomePrices PolyOutcomePrices `json:"outcomePrices"`
	Volume        PolyFloat         `json:"volume"`
	Active        bool              `json:"active"`
	Closed        bool              `json:"closed"`
	CreatedAt     string            `json:"createdAt"`
	UpdatedAt     string            `json:"updatedAt"`
}

type PolyTag struct {
	TagID PolyInt `json:"id"`
	Label string  `json:"label"`
}

type PolyInt int

func (pi *PolyInt) UnmarshalJSON(data []byte) error {
	var void interface{}
	if err := json.Unmarshal(data, &void); err != nil {
		return err
	}

	switch t := void.(type) {
	case string:
		// "500"
		i, err := strconv.Atoi(t)
		if err != nil {
			// "500.0"
			f, fErr := strconv.ParseFloat(t, 64)
			if fErr != nil {
				return fErr
			}
			*pi = PolyInt(int(f))
			return nil
		}
		*pi = PolyInt(i)
		return nil

	case float64:
		*pi = PolyInt(int(t))
		return nil

	case nil:
		*pi = PolyInt(0)
		return nil

	default:
		return fmt.Errorf("Error parsing into PolyInt: Type %T", t)
	}
}

type PolyFloat float32

func (pf *PolyFloat) UnmarshalJSON(data []byte) error {
	var void interface{}
	if err := json.Unmarshal(data, &void); err != nil {
		return err
	}

	switch t := void.(type) {
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return err
		}
		*pf = PolyFloat(f)
		return nil

	case float64:
		*pf = PolyFloat(t)
		return nil

	case nil:
		*pf = 0
		return nil

	default:
		return fmt.Errorf("Error parsing into PolyFloat: Type %T", t)
	}
}

type PolyOutcomes []string

func (po *PolyOutcomes) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	var tempSlice []string
	if err := json.Unmarshal([]byte(s), &tempSlice); err != nil {
		return err
	}

	*po = tempSlice
	return nil
}

type PolyOutcomePrices []PolyFloat

func (pop *PolyOutcomePrices) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	var tempSlice []PolyFloat
	if err := json.Unmarshal([]byte(s), &tempSlice); err != nil {
		return err
	}

	*pop = tempSlice
	return nil
}

// Client Models
type ClientEvent struct {
	ID          int            `json:"id"`
	EventID     PolyInt        `json:"event_id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Markets     []ClientMarket `json:"markets"`
	Tags        []string       `json:"tags"`
}

type ClientMarket struct {
	ID            int               `json:"id"`
	MarketID      PolyInt           `json:"market_id"`
	Question      string            `json:"question"`
	Description   string            `json:"description"`
	Outcomes      PolyOutcomes      `json:"outcomes"`
	OutcomePrices PolyOutcomePrices `json:"outcome_prices"`
}

func (pe *PolyEvent) ToClient() ClientEvent {

	markets := make([]ClientMarket, len(pe.Markets))
	for i, m := range pe.Markets {
		markets[i] = m.ToClient()
	}

	tags := make([]string, len(pe.Tags))
	for i, t := range pe.Tags {
		tags[i] = t.Label
	}

	description := strings.ReplaceAll(pe.Description, "\n", "")

	return ClientEvent{
		ID:          0,
		EventID:     pe.EventID,
		Title:       pe.Title,
		Description: description,
		Markets:     markets,
		Tags:        tags,
	}
}

func (pm *PolyMarket) ToClient() ClientMarket {
	return ClientMarket{
		ID:            0,
		MarketID:      pm.MarketID,
		Question:      pm.Question,
		Description:   pm.Description,
		Outcomes:      pm.Outcomes,
		OutcomePrices: pm.OutcomePrices,
	}
}
