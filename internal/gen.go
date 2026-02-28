package internal

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"google.golang.org/genai"
)

type GenaiConfig struct {
	GeminiKey 					string
	EmbeddingTaskType 	string
	EmbedModelName      string
	LanguageModelName		string
	LanguageTemperature float32
}

func NewGenaiConfig() *GenaiConfig {
	return &GenaiConfig{
		GeminiKey: os.Getenv("GEMINI_KEY"),
		EmbedModelName: "gemini-embedding-001",
		EmbeddingTaskType: "SEMANTIC_SIMILARITY",
		LanguageModelName: "gemini-3-flash-preview",
		LanguageTemperature: 0.5,
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

func (c *GenaiConfig) DiscoverArbitrage(ctx context.Context, client *genai.Client, events []DiscoveryEvent) error {
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
		Temperature: &c.LanguageTemperature,
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
		genai.NewContentFromText("Find arbitrages in these markets", genai.RoleUser),
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