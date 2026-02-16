package provider

import "testing"

func TestResolveModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		requestModel string
		defaultModel string
		want         string
	}{
		{
			name:         "uses request model",
			requestModel: "gpt-4o-mini",
			defaultModel: "gpt-4.1",
			want:         "gpt-4o-mini",
		},
		{
			name:         "trims request model",
			requestModel: "  llama3.2  ",
			defaultModel: "llama3.1",
			want:         "llama3.2",
		},
		{
			name:         "falls back when request model empty",
			requestModel: "",
			defaultModel: "qwen2.5",
			want:         "qwen2.5",
		},
		{
			name:         "falls back when request model whitespace",
			requestModel: "   ",
			defaultModel: "gemini-2.0-flash",
			want:         "gemini-2.0-flash",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := resolveModel(tc.requestModel, tc.defaultModel)
			if got != tc.want {
				t.Fatalf("resolveModel(%q, %q) = %q, want %q", tc.requestModel, tc.defaultModel, got, tc.want)
			}
		})
	}
}
