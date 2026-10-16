package validate

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// FeaturedChapaMeta is the meta block Chapa returns on featured-campaign payments.
type FeaturedChapaMeta struct {
	BidID        string `json:"bid_id"`
	BidIDCamel   string `json:"bidId"`
	AgentID      string `json:"agent_id"`
	AgentIDCamel string `json:"agentId"`
	Placement    string `json:"placement"`
	Days         string `json:"days"`
	DaysInt      int    `json:"-"`
}

// ParseFeaturedChapaMeta normalizes Chapa verify meta into campaign fields.
func ParseFeaturedChapaMeta(meta interface{}) (FeaturedChapaMeta, error) {
	raw := metaToMap(meta)
	b, err := json.Marshal(raw)
	if err != nil {
		return FeaturedChapaMeta{}, err
	}
	var m FeaturedChapaMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return FeaturedChapaMeta{}, err
	}
	m.BidID = FirstNonEmpty(m.BidID, m.BidIDCamel)
	m.AgentID = FirstNonEmpty(m.AgentID, m.AgentIDCamel)
	m.DaysInt = 1
	if ds := strings.TrimSpace(m.Days); ds != "" {
		if n, err := strconv.Atoi(ds); err == nil && n > 0 {
			m.DaysInt = n
		}
	}
	if m.BidID == "" || m.AgentID == "" {
		return m, fmt.Errorf("missing bid_id or agent_id in chapa meta")
	}
	if _, err := uuid.Parse(m.BidID); err != nil {
		return m, fmt.Errorf("invalid bid_id in chapa meta")
	}
	if _, err := uuid.Parse(m.AgentID); err != nil {
		return m, fmt.Errorf("invalid agent_id in chapa meta")
	}
	return m, nil
}

func metaToMap(meta interface{}) map[string]interface{} {
	if meta == nil {
		return map[string]interface{}{}
	}
	if m, ok := meta.(map[string]interface{}); ok {
		return m
	}
	if s, ok := meta.(string); ok {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(s), &m); err == nil && m != nil {
			return m
		}
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return map[string]interface{}{}
	}
	return m
}
