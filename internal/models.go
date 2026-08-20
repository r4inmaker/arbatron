package internal

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Polymarket Models
type PolyEvent struct {
	ID           int
	EventID      PolyInt       `json:"id"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	StartDate    PolyTimestamp `json:"startDate"`
	CreationDate PolyTimestamp `json:"creationDate"`
	EndDate      PolyTimestamp `json:"endDate"`
	Active       bool          `json:"active"`
	Closed       bool          `json:"closed"`
	Archived     bool          `json:"archived"`
	Volume       PolyFloat     `json:"volume"`
	Category     string        `json:"category"`
	PublishedAt  PolyTimestamp `json:"published_at"`
	CreatedAt    PolyTimestamp `json:"createdAt"`
	UpdatedAt    string        `json:"updatedAt"`
	Markets      []PolyMarket  `json:"markets"`
	Tags         []PolyTag     `json:"tags"`
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

// Client Models
type ClientEvent struct {
	ID          int            `json:"id"`
	EventID     int            `json:"event_id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Markets     []ClientMarket `json:"markets"`
	Volume      PolyFloat      `json:"volume"`
	Tags        []string       `json:"tags"`
	StartDate   PolyTimestamp  `json:"start_date"`
	EndDate     PolyTimestamp  `json:"end_date"`
}

type ClientMarket struct {
	ID            int               `json:"id"`
	MarketID      int               `json:"market_id"`
	Question      string            `json:"question"`
	Description   string            `json:"description"`
	Outcomes      PolyOutcomes      `json:"outcomes"`
	OutcomePrices PolyOutcomePrices `json:"outcome_prices"`
}

type PolyTag struct {
	TagID PolyInt `json:"id"`
	Label string  `json:"label"`
}

// Database Models
type DBEvent struct {
	ID        int
	EventID   int
	Title     string
	Embedding string
	StartDate PolyTimestamp
	EndDate   PolyTimestamp
}

// Discovery Models (LLM schema)
type DiscoveryEvent struct {
	EventID int64								`json:"event_id"`		
	Title  	string							`json:"title"`
	Markets []DiscoveryMarket		`json:"markets"`
}

type DiscoveryMarket struct {
	MarketID 			int64  							`json:"market_id"`
	Question 			string							`json:"question"`
	Description 	string							`json:"description"`
	Outcomes			PolyOutcomes				`json:"outcomes"`
	OutcomePrices PolyOutcomePrices		`json:"outcome_prices"`
}

type ClusteringEvent struct {
    EventID  int64              `json:"event_id"`
    Title    string             `json:"title"`
    Markets  []ClusteringMarket `json:"markets"`
}

type ClusteringMarket struct {
    MarketID int64  `json:"market_id"`
    Question string `json:"question"`
}


type GenaiClusteringMarketRef struct {
    MarketID    int64  `json:"market_id"`
    Question    string `json:"market_title"`
}

type GenaiArbitrageCluster struct {
    Markets         []GenaiClusteringMarketRef `json:"markets"`
    Reasoning       string                `json:"reasoning"`
}

type GenaiClusteringResponse struct {
    ArbitrageClusters []GenaiArbitrageCluster `json:"arbitrage_clusters"`
}

type GenaiArbitrageMarket struct {
    MarketID float64 			`json:"market_id"`
    Title    string  			`json:"title"`
    Outcome  string  			`json:"outcome"`
		OutcomePrice float32	`json:"outcome_price"`
}

type GenaiArbitrage struct {
    Title         []string              `json:"title"`
    ROI           float64               `json:"roi"`
    Reasoning     string                `json:"reasoning"`
    Risk          float64               `json:"risk"`
    ArbitrageType string                `json:"arbitrage_type"`
    Markets       []GenaiArbitrageMarket `json:"markets"`
}


type GenaiArbitrageResponse struct {
    Arbitrages []GenaiArbitrage `json:"arbitrages"`
}

func (pe *PolyEvent) ToDiscovery() DiscoveryEvent {
	markets := make([]DiscoveryMarket, len(pe.Markets))
	for i, m := range pe.Markets {
		markets[i] = m.ToDiscovery()
	}

	return DiscoveryEvent{
		EventID: int64(pe.EventID),
		Title: pe.Title,
		Markets: markets,
	}
}

func (pm *PolyMarket) ToDiscovery() DiscoveryMarket {
	return DiscoveryMarket{
		MarketID: int64(pm.MarketID),
		Question: pm.Question,
		Description: pm.Description,
		Outcomes: pm.Outcomes,
		OutcomePrices: pm.OutcomePrices,
	}
}

func (pe *PolyEvent) ToClustering() ClusteringEvent {
    markets := make([]ClusteringMarket, len(pe.Markets))
    for i, m := range pe.Markets {
        markets[i] = m.ToClustering()
    }
    return ClusteringEvent{
        EventID: int64(pe.EventID),
        Title:   pe.Title,
        Markets: markets,
    }
}

func (pm *PolyMarket) ToClustering() ClusteringMarket {
    return ClusteringMarket{
        MarketID: int64(pm.MarketID),
        Question: pm.Question,
    }
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
		EventID:     int(pe.EventID),
		Title:       pe.Title,
		Description: description,
		Volume:      pe.Volume,
		Markets:     markets,
		Tags:        tags,
		StartDate:   pe.StartDate,
		EndDate:     pe.EndDate,
	}
}

func (pm *PolyMarket) ToClient() ClientMarket {
	return ClientMarket{
		ID:            0,
		MarketID:      int(pm.MarketID),
		Question:      pm.Question,
		Description:   pm.Description,
		Outcomes:      pm.Outcomes,
		OutcomePrices: pm.OutcomePrices,
	}
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

type PolyTimestamp time.Time

func (pt *PolyTimestamp) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	// Parse different layouts since the API is inconsistent
	layouts := []string{
		time.RFC3339,                 // 2021-08-11T00:00:00Z
		"2006-01-02 15:04:05.999-07", // 2022-07-27 14:40:10.891+00
		"2006-01-02 15:04:05",        // Basic fallback
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			*pt = PolyTimestamp(t)
			return nil
		}
	}

	return fmt.Errorf("unable to parse time from any provided layouts: %s", s)
}
