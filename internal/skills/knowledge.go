package skills

import (
	"context"
	"fmt"

	"github.com/nobodycan/digital-twin/internal/llm"
	"github.com/nobodycan/digital-twin/internal/store"
	"github.com/nobodycan/digital-twin/pkg/types"
)

// EmbedSkill embeds text through the LLM abstraction.
type EmbedSkill struct{ client llm.Client }

func NewEmbedSkill(client llm.Client) EmbedSkill { return EmbedSkill{client: client} }
func (s EmbedSkill) Name() string                { return "embed" }

func (s EmbedSkill) Run(ctx context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{{Name: "text", Type: String, Required: true}}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	var vector []float64
	if s.client != nil {
		vector, err = s.client.Embed(ctx, valid["text"].(string))
		if err != nil {
			return types.SkillResult{}, err
		}
	}
	return types.SkillResult{SkillName: s.Name(), Output: vector}, nil
}

// VectorSearchSkill searches vector documents.
type VectorSearchSkill struct{ store store.VectorStore }

func NewVectorSearchSkill(vectorStore store.VectorStore) VectorSearchSkill {
	return VectorSearchSkill{store: vectorStore}
}
func (s VectorSearchSkill) Name() string { return "vector_search" }

func (s VectorSearchSkill) Run(ctx context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{
		{Name: "vector", Type: Object, Required: true},
		{Name: "limit", Type: Number, Default: 3.0},
	}}).Validate(normalizeVectorParams(params))
	if err != nil {
		return types.SkillResult{}, err
	}
	vector := valid["vector"].(map[string]any)["values"].([]float64)
	var results []store.VectorSearchResult
	if s.store != nil {
		results, err = s.store.Search(ctx, vector, int(valid["limit"].(float64)))
		if err != nil {
			return types.SkillResult{}, err
		}
	}
	return types.SkillResult{SkillName: s.Name(), Output: results}, nil
}

// CiteSkill formats source citations.
type CiteSkill struct{}

func NewCiteSkill() CiteSkill { return CiteSkill{} }
func (s CiteSkill) Name() string {
	return "cite"
}

func (s CiteSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{
		{Name: "source_id", Type: String, Required: true},
		{Name: "content", Type: String, Required: true},
	}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	return types.SkillResult{
		SkillName: s.Name(),
		Output:    fmt.Sprintf("[%s] %s", valid["source_id"], valid["content"]),
	}, nil
}

func normalizeVectorParams(params map[string]any) map[string]any {
	out := make(map[string]any, len(params))
	for key, value := range params {
		out[key] = value
	}
	switch values := params["vector"].(type) {
	case []float64:
		out["vector"] = map[string]any{"values": values}
	case []any:
		vector := make([]float64, 0, len(values))
		for _, item := range values {
			switch v := item.(type) {
			case float64:
				vector = append(vector, v)
			case int:
				vector = append(vector, float64(v))
			default:
				return out
			}
		}
		out["vector"] = map[string]any{"values": vector}
	}
	return out
}
