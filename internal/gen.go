package internal

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"google.golang.org/genai"
)

type GenaiConfig struct {
	GeminiKey 						string
	DeepSeekKey 					string
	EmbeddingTaskType 		string
	EmbedModelName      	string
	LanguageModelName			string
	ClusteringTemperature float32
	ArbitrageTemperature 	float32
}

func NewGenaiConfig() *GenaiConfig {
	return &GenaiConfig{
		GeminiKey: os.Getenv("GEMINI_KEY"),
		DeepSeekKey: os.Getenv("DEEPSEEK_KEY"),
		EmbedModelName: "gemini-embedding-001",
		EmbeddingTaskType: "SEMANTIC_SIMILARITY",
		LanguageModelName: "gemini-2.5-pro",
		ClusteringTemperature: 0.5,
		ArbitrageTemperature: 0.1,
	}
}

func (c *GenaiConfig) GenerateEmbeddings(ctx context.Context, client *genai.Client, payload []string) ([]string, error) {
	
	contents := make([]*genai.Content, len(payload))
	for i, p := range payload {
		contents[i] = genai.NewContentFromText(p, genai.RoleUser)
	}

	result, err := client.Models.EmbedContent(ctx, c.EmbedModelName, contents,
		&genai.EmbedContentConfig{
			TaskType: c.EmbeddingTaskType,
			OutputDimensionality: genai.Ptr(int32(768)),
		},
	)

	if err != nil {
		return nil, err
	}

	var embeddings []string
	for i, _ := range payload {
		embeddings = append(embeddings, VecToString(result.Embeddings[i].Values))
	}

	return embeddings, nil
}

func (c *GenaiConfig) DiscoverArbitrageClusters(ctx context.Context, client *genai.Client, events []ClusteringEvent) error {
	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"arbitrage_clusters" : &genai.Schema{
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"markets" : {
							Type: genai.TypeArray,
							Items: &genai.Schema{
								Type: genai.TypeObject,
								Properties: map[string]*genai.Schema{
									"market_id": {Type: genai.TypeNumber},
									"market_title": {Type: genai.TypeString},
								},
							},
						},
						"reasoning" : {Type: genai.TypeString},
					},
				},
			},
		},
	}

	sysPrompt := `
		You are a prediction market analyst specializing in identifying arbitrage opportunities on Polymarket.

		You will receive a list of events, each containing markets with their IDs and questions only. No prices are provided at this stage.

		Your job is to group markets into clusters that are strong candidates for arbitrage based on their logical structure alone.

		Look for:

		COMBINATORIAL_INSIDE_EVENT: Multiple markets within the same event that are genuinely mutually exclusive and exhaustive — only ONE can resolve Yes. Examples: "Who wins the election: Alice?", "Who wins the election: Bob?" — only one candidate can win.

		CROSS_EVENT: Markets across different events that are logically linked or dependent. Think creatively and deeply:
		- Causal: if X happens, Y becomes impossible or certain
		- Negation: "Will X win?" vs "Will X lose?" framed across different events
		- Conditional: one market resolving Yes makes another's pricing irrational
		- Temporal: an earlier event that makes a later one redundant or guaranteed
		- Shared outcome: two differently-framed markets that must resolve the same way
		- Mutual exclusion across events: e.g. "Will Alice be the Fed chair?" and "Will Bob be the Fed chair?" from separate events
		Do not limit yourself to obvious connections — surface subtle logical dependencies the market may have mispriced.

		CRITICAL RULES:
		- Daily occurrence markets ("Will X happen on Monday?", "Will X happen on Tuesday?") are NOT mutually exclusive. The same event can happen on multiple days. Never cluster these as COMBINATORIAL.
		- Only return clusters you are highly confident have a real structural arbitrage opportunity.
		- If you find nothing, return an empty arbitrage_clusters array. Do not invent opportunities.
	`


		config := &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema: schema,
			Temperature: &c.ClusteringTemperature,
			SystemInstruction: genai.NewContentFromText(sysPrompt, genai.RoleUser),
		}

		contents := make([]*genai.Part, len(events))
		for i, ev := range events {
			var payload strings.Builder
			if err := json.NewEncoder(&payload).Encode(ev); err != nil {
				return err
			}
			contents[i] = genai.NewPartFromText(payload.String())
		}

		userContent := []*genai.Content{
			genai.NewContentFromText("Find potential clusters in these markets:", genai.RoleUser),
			genai.NewContentFromParts(contents, genai.RoleUser),
		}

		result, err := client.Models.GenerateContent(
			ctx,
			c.LanguageModelName,
			userContent,
			config,
		)

		if err != nil {
			return err
		}

		var response GenaiClusteringResponse
		if err := json.Unmarshal([]byte(result.Text()), &response); err != nil {
			return err
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(response)

	return nil
}

func (c *GenaiConfig) DiscoverArbitrageInMarkets(ctx context.Context, client *genai.Client, events []DiscoveryEvent) error {
	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"arbitrages" : {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"title":     {Type: genai.TypeArray, 
													 Items: &genai.Schema{
														Type: genai.TypeString,
														},
													},
						"roi": 				{Type: genai.TypeNumber},
						"reasoning": 	{Type: genai.TypeString},
						"risk":				{Type: genai.TypeNumber, Minimum: genai.Ptr(0.0), Maximum: genai.Ptr(1.0)},
						"arbitrage_type": {Type: genai.TypeString, 
															 Enum: []string{"intra_market", "combinatorial_inside_event", "cross_event"},},
						"markets" : {
							Type: genai.TypeArray,
							Items: &genai.Schema{
								Type: genai.TypeObject,
								Properties: map[string]*genai.Schema{
									"market_id": 			{Type: genai.TypeNumber},
									"title"		 : 			{Type: genai.TypeString}, 
									"outcome"  : 			{Type: genai.TypeString},
									"outcome_price": 	{Type: genai.TypeNumber},
								},
							},
						},
					},	
				},
			},
		},
	}

	sysPrompt := `
		You are a semantic aribtrage identifier for Polymarket markets.
		You will receive a batch of events with nested markets. 
		Your job is to analyze them and find arbitrage opportunities in them.
		Keep in mind that while you do have to minimize risk, there is a certain
		transaction fee, so prioritize arbitrages with larger roi.
		You combine them in groups of at least 1 (for intra_market arbitrages)
		and at most 5 for combinatorial arbitrages.
		In the outcome field you will insert what outcome to buy for each of the
		markets to have an arbitrage position.
		You will include the roi of a potential arbitrage, type of arbitrage
		and reasoning for why yout think this is a solid arbitrage.
		If you dont find any arbitrage opportunities, do not make them up,
		return an empty response instead.
	`

	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: schema,
		Temperature: &c.ArbitrageTemperature,
		SystemInstruction: genai.NewContentFromText(sysPrompt, genai.RoleUser),
	}

	contents := make([]*genai.Part, len(events))
	for i, ev := range events {
		var payload strings.Builder
		if err := json.NewEncoder(&payload).Encode(ev); err != nil {
			return err
		}
		contents[i] = genai.NewPartFromText(payload.String())
	}

	userContent := []*genai.Content{
		genai.NewContentFromText("Find arbitrages in these markets:", genai.RoleUser),
		genai.NewContentFromParts(contents, genai.RoleUser),
	}

	result, err := client.Models.GenerateContent(
		ctx,
		c.LanguageModelName,
		userContent,
		config,
	)

	if err != nil {
		return err
	}
	
	var response GenaiArbitrageResponse
	if err := json.Unmarshal([]byte(result.Text()), &response); err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(response)
	return nil
} 