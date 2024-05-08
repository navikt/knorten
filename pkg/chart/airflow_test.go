package chart

import "testing"

func TestTeamID(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "non-empty",
			input:    "foo",
			expected: "foo",
		},
		{
			name:     "team- prefix",
			input:    "team-bar",
			expected: "bar",
		},
		{
			name:     "team prefix",
			input:    "teambar",
			expected: "bar",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := teamIDToDb(tc.input)
			if got != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, got)
			}
		})
	}
}
