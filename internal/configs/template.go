package configs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"
)

type TemplateVisibility string

const (
	TemplateOrganization TemplateVisibility = "organization"
	TemplateSelected     TemplateVisibility = "selected-groups"
)

type Template struct {
	ID                    string             `json:"id"`
	Name                  string             `json:"name"`
	Version               string             `json:"version"`
	Description           string             `json:"description"`
	Content               string             `json:"content"`
	ContentHash           string             `json:"content_hash"`
	Visibility            TemplateVisibility `json:"visibility"`
	GroupIDs              []string           `json:"group_ids"`
	SourceConfigurationID string             `json:"source_configuration_id,omitempty"`
	CreatedBy             string             `json:"created_by"`
	CreatedAt             time.Time          `json:"created_at"`
}

func NewTemplate(name, version, description, content string, visibility TemplateVisibility, groupIDs []string, sourceConfigurationID, createdBy string, createdAt time.Time) (*Template, error) {
	name, version = strings.TrimSpace(name), strings.TrimSpace(version)
	if name == "" || version == "" || strings.TrimSpace(content) == "" {
		return nil, errors.New("template name, version and content are required")
	}
	switch visibility {
	case TemplateOrganization:
		groupIDs = nil
	case TemplateSelected:
		groupIDs = normalizeTemplateGroups(groupIDs)
		if len(groupIDs) == 0 {
			return nil, errors.New("selected-group templates require at least one group")
		}
	default:
		return nil, errors.New("unsupported template visibility")
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	contentSum := sha256.Sum256([]byte(content))
	identity := strings.Join([]string{name, version, content, string(visibility), strings.Join(groupIDs, "\x1f")}, "\x00")
	idSum := sha256.Sum256([]byte(identity))
	return &Template{
		ID: hex.EncodeToString(idSum[:]), Name: name, Version: version, Description: strings.TrimSpace(description),
		Content: content, ContentHash: hex.EncodeToString(contentSum[:]), Visibility: visibility,
		GroupIDs: groupIDs, SourceConfigurationID: strings.TrimSpace(sourceConfigurationID),
		CreatedBy: strings.TrimSpace(createdBy), CreatedAt: createdAt.UTC(),
	}, nil
}

func (t *Template) AllowsGroup(groupID string) bool {
	if t == nil {
		return false
	}
	if t.Visibility == TemplateOrganization {
		return true
	}
	for _, allowed := range t.GroupIDs {
		if allowed == groupID {
			return true
		}
	}
	return false
}

func normalizeTemplateGroups(groupIDs []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(groupIDs))
	for _, id := range groupIDs {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}

func CloneTemplate(t *Template) *Template {
	if t == nil {
		return nil
	}
	copy := *t
	copy.GroupIDs = append([]string(nil), t.GroupIDs...)
	return &copy
}
