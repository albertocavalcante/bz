package modulearg

import "testing"

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input       string
		wantName    string
		wantVersion string
	}{
		{input: "rules_go@0.50.1", wantName: "rules_go", wantVersion: "0.50.1"},
		{input: "rules_go", wantName: "rules_go", wantVersion: ""},
		{input: "@scope/pkg", wantName: "@scope/pkg", wantVersion: ""},
		{input: "@scope/pkg@1.2.3", wantName: "@scope/pkg", wantVersion: "1.2.3"},
		{input: "pkg@1.0.0-rc1", wantName: "pkg", wantVersion: "1.0.0-rc1"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			gotName, gotVersion := Parse(tt.input)
			if gotName != tt.wantName || gotVersion != tt.wantVersion {
				t.Fatalf("Parse(%q) = (%q, %q), want (%q, %q)", tt.input, gotName, gotVersion, tt.wantName, tt.wantVersion)
			}
		})
	}
}
