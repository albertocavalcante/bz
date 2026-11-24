package cmd

import (
	"context"

	"github.com/albertocavalcante/bz/internal/osv"
)

// mockOSV answers queries from a map keyed "name@version" or "name".
type mockOSV struct {
	Vulnerabilities map[string][]osv.Vulnerability
	Error           error
}

func (m *mockOSV) Query(_ context.Context, name, version, _ string) ([]osv.Vulnerability, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if v, ok := m.Vulnerabilities[name+"@"+version]; ok {
		return v, nil
	}
	return m.Vulnerabilities[name], nil
}
