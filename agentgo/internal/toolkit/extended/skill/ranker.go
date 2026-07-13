package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"agentgo/internal/model"
	"agentgo/internal/provider"
)

// SkillRecommendation is a single ranked match with an LLM-generated reason.
type SkillRecommendation struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
	Scenario    string `json:"scenario,omitempty"`
	HasAssets   bool   `json:"has_assets"`
	HasPreview  bool   `json:"has_preview"`
}

// rankerExcluded lists skills that should never appear in recommendations,
// regardless of HasAssets. html-ppt is excluded because its assets/ directory
// causes the loader to set HasAssets=true, but it is a template, not a
// selectable design skill.
var rankerExcluded = map[string]bool{"html-ppt": true}

const rankerSystemPrompt = `You are a design skill matcher. Based on the user's request, select the most relevant design skills from the candidate list.
Return a JSON object with a "picks" array: [{"index": 0, "reason": "why this matches"}].
Return at most the requested number of picks. Return {"picks": []} if nothing matches.
Do not return any text outside the JSON.`

// SkillRanker uses an LLM to select relevant design skills from a candidate pool.
type SkillRanker struct {
	idx *SkillIndex
}

// NewSkillRanker creates a new SkillRanker backed by the given SkillIndex.
func NewSkillRanker(idx *SkillIndex) *SkillRanker {
	return &SkillRanker{idx: idx}
}

// Recommend runs an LLM call to pick up to limit skills matching query + designBrief.
func (r *SkillRanker) Recommend(ctx context.Context, p provider.StreamingProvider, query, designBrief string, limit int) ([]SkillRecommendation, error) {
	candidates := r.buildCandidatePool()
	if len(candidates) == 0 {
		return nil, nil
	}

	reqText := query
	if designBrief != "" {
		reqText += "\nRefined preferences: " + designBrief
	}

	var sb strings.Builder
	for i, c := range candidates {
		fmt.Fprintf(&sb, "[%d] %s — %s\n", i, c.Name, c.Description)
	}

	userMsg := fmt.Sprintf("User request: %s\n\nDesign skills:\n%s", reqText, sb.String())

	resp, err := p.Chat(ctx, provider.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: rankerSystemPrompt},
			{Role: "user", Content: userMsg},
		},
		Stream:    false,
		MaxTokens: 256,
	})
	if err != nil {
		return nil, fmt.Errorf("skill ranker: %w", err)
	}

	picks := parseRankerResponse(resp.Message.Content)
	recs := make([]SkillRecommendation, 0, len(picks))
	for _, pick := range picks {
		if pick.index < 0 || pick.index >= len(candidates) {
			continue
		}
		c := candidates[pick.index]
		recs = append(recs, SkillRecommendation{
			Name:        c.Name,
			Description: c.Description,
			Reason:      pick.reason,
			Scenario:    c.Scenario,
			HasAssets:   c.HasAssets,
			HasPreview:  c.HasPreview,
		})
	}
	return recs, nil
}

type candidateInfo struct {
	Name        string
	Description string
	Scenario    string
	HasAssets   bool
	HasPreview  bool
}

func (r *SkillRanker) buildCandidatePool() []candidateInfo {
	deckSkills := r.idx.DeckSkills()
	candidates := make([]candidateInfo, 0, len(deckSkills))
	for _, sk := range deckSkills {
		if rankerExcluded[sk.Name] {
			continue
		}
		desc := sk.Description
		if len(desc) > 120 {
			desc = desc[:120]
		}
		candidates = append(candidates, candidateInfo{
			Name:        sk.Name,
			Description: desc,
			Scenario:    sk.Scenario,
			HasAssets:   sk.HasAssets,
			HasPreview:  sk.HasPreview,
		})
	}
	return candidates
}

type rankerPick struct {
	index  int
	reason string
}

func parseRankerResponse(content string) []rankerPick {
	content = strings.TrimSpace(content)

	// Strip markdown code fences.
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var parsed struct {
		Picks []struct {
			Index  int    `json:"index"`
			Reason string `json:"reason"`
		} `json:"picks"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		// Fallback: try comma-separated numbers (like parseRecallResponse).
		return parseRankerFallback(content)
	}

	var picks []rankerPick
	for _, p := range parsed.Picks {
		if p.Index < 0 {
			continue
		}
		picks = append(picks, rankerPick{index: p.Index, reason: p.Reason})
	}
	return picks
}

func parseRankerFallback(content string) []rankerPick {
	parts := strings.FieldsFunc(content, func(r rune) bool {
		return r == ',' || r == ' ' || r == '，'
	})
	var picks []rankerPick
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if n, err := strconv.Atoi(p); err == nil && n >= 0 {
			picks = append(picks, rankerPick{index: n})
		}
	}
	return picks
}
