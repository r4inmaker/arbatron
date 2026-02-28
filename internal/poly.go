package internal

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// If we have smaller clusters it makes sense to send events along with markets,
// because the model might not be able to accurately identify arbs just based
// on titles.
// One stage pipeline:
// + Higher chance of identifying certain arbs
// - Might miss out on some not-obvious arbs (they might not be that close in embedding distance)

// If we send larger clusters, we dont want to include the market data
// because the model has finite attention / context window
// Two stage pipeline
// + Might find some unusual arbs
// - Might miss out on some arbs because there wont be market data in the first request

// Identify the optimal context length and decide on the approach / write two pipelines
// one for small clusters and one for large clusters

// Limits
// Reasoning: 200_000 tokens or after context is 80% full
//

func GetMarketsFromEventID(eventId int64) (DiscoveryEvent, error) {
	eventURL, err := url.JoinPath("https://gamma-api.polymarket.com/events", strconv.FormatInt(eventId, 10))
	if err != nil {
		return DiscoveryEvent{}, err
	}

	resp, err := http.Get(eventURL)
	if err != nil {
		return DiscoveryEvent{}, err
	}
	defer resp.Body.Close()

	var polyEvent PolyEvent
	if err := json.NewDecoder(resp.Body).Decode(&polyEvent); err != nil {
		return DiscoveryEvent{}, err
	}

	return polyEvent.ToDiscovery(), nil
}