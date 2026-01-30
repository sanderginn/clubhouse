package handlers

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractMentionedUsernames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:     "basic ascii mentions",
			input:    "hi @alice and @bob_2",
			expected: []string{"alice", "bob_2"},
		},
		{
			name:     "deduplicate",
			input:    "@alice hi @alice again",
			expected: []string{"alice"},
		},
		{
			name:     "unicode usernames",
			input:    "hey @mårten and @用户一 are here",
			expected: []string{"mårten", "用户一"},
		},
		{
			name:     "ignore inside word",
			input:    "email foo@bar.com and hi@alice",
			expected: nil,
		},
		{
			name:     "ignore escaped mentions",
			input:    "literal \\@alice and mention @bob",
			expected: []string{"bob"},
		},
		{
			name:     "min length",
			input:    "short @ab ok @abc",
			expected: []string{"abc"},
		},
		{
			name:     "max length",
			input:    "@" + strings.Repeat("a", 50) + " @" + strings.Repeat("b", 51),
			expected: []string{strings.Repeat("a", 50)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := extractMentionedUsernames(test.input)
			if !reflect.DeepEqual(got, test.expected) {
				t.Fatalf("expected %v, got %v", test.expected, got)
			}
		})
	}
}
