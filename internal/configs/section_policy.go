package configs

// SectionPolicy controls whether Operators may edit a Collector configuration
// section. Admins always retain edit access; Viewers remain read-only.
type SectionPolicy struct {
	SectionKey       string `json:"section_key"`
	OperatorEditable bool   `json:"operator_editable"`
}

// DefaultSectionPolicies returns secure defaults. Export destinations and
// service wiring require Admin access unless an Admin explicitly delegates them.
func DefaultSectionPolicies() []SectionPolicy {
	result := make([]SectionPolicy, 0, len(ConfigurationSectionDefinitions))
	for _, definition := range ConfigurationSectionDefinitions {
		result = append(result, SectionPolicy{
			SectionKey: definition.Key, OperatorEditable: definition.DefaultOperatorEditable,
		})
	}
	return result
}
