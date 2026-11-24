package mod

import "testing"

func TestResolveWorkflowName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		flag    string
		args    []string
		want    string
		wantErr bool
	}{
		{name: "flag only", flag: "mirror", want: "mirror"},
		{name: "positional only", args: []string{"mirror"}, want: "mirror"},
		{name: "both agree", flag: "mirror", args: []string{"mirror"}, want: "mirror"},
		{name: "both differ", flag: "a", args: []string{"b"}, wantErr: true},
		{name: "neither", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveWorkflowName(tt.flag, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
