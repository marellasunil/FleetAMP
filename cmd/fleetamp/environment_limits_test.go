package main

import "testing"

func TestBoundedIntEnvOrDefault(t *testing.T) {
	const key = "FLEETAMP_TEST_BOUNDED_INT"
	for _, test := range []struct {
		name, value string
		want        int
	}{
		{name: "valid", value: "7", want: 7},
		{name: "below minimum", value: "0", want: 3},
		{name: "above maximum", value: "11", want: 3},
		{name: "integer overflow", value: "9223372036854775807", want: 3},
		{name: "invalid", value: "many", want: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(key, test.value)
			if got := boundedIntEnvOrDefault(key, 3, 1, 10); got != test.want {
				t.Fatalf("boundedIntEnvOrDefault()=%d want %d", got, test.want)
			}
		})
	}
}
