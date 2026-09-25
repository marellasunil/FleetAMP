package configs

import "testing"

func TestParseDriftPolicy(t *testing.T) {
	for _, value := range []string{string(DriftPolicyReport), string(DriftPolicyEnforce)} {
		policy, err := ParseDriftPolicy(value)
		if err != nil || string(policy) != value {
			t.Fatalf("ParseDriftPolicy(%q) = %q, %v", value, policy, err)
		}
	}
	if _, err := ParseDriftPolicy("overwrite"); err == nil {
		t.Fatal("expected invalid drift policy to fail")
	}
}
