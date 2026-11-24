package envutil

import "testing"

func TestIsTruthy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want bool
	}{
		{in: "1", want: true},
		{in: "true", want: true},
		{in: "TRUE", want: true},
		{in: "yes", want: true},
		{in: " YeS ", want: true},
		{in: "", want: false},
		{in: "0", want: false},
		{in: "false", want: false},
		{in: "no", want: false},
		{in: "random", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if got := IsTruthy(tt.in); got != tt.want {
				t.Fatalf("IsTruthy(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      string
		want    bool
		wantErr bool
	}{
		{in: "1", want: true},
		{in: "TRUE", want: true},
		{in: "yes", want: true},
		{in: " On ", want: true},
		{in: "", want: false},
		{in: "0", want: false},
		{in: "False", want: false},
		{in: "no", want: false},
		{in: "off", want: false},
		{in: "ture", wantErr: true},
		{in: "2", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := ParseBool(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseBool(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("ParseBool(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsTruthyEnv(t *testing.T) {
	const envName = "BZ_ENVUTIL_TEST"
	t.Setenv(envName, "yes")
	if !IsTruthyEnv(envName) {
		t.Fatalf("IsTruthyEnv(%q) = false, want true", envName)
	}
}
