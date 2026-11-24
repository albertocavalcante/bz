package publisher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestNewHTTPPublisher_RejectsUnsafeBase(t *testing.T) {
	t.Parallel()

	for _, base := range []string{
		"https://example.com/base?tok=1",
		"https://example.com/base?",
		"https://example.com/base#frag",
		"https://example.com/base#",
		"ftp://example.com/base",
		"example.com/base",
		"https:///nohost",
		"https://u:s3cret@example.com/base?tok=1",
		"https://u:s3cret@exa mple.com/base",
	} {
		_, err := NewHTTPPublisher(base, nil)
		if err == nil {
			t.Errorf("NewHTTPPublisher(%q) succeeded, want an error", base)
			continue
		}
		if strings.Contains(err.Error(), "s3cret") {
			t.Errorf("NewHTTPPublisher(%q) error leaks the password: %v", base, err)
		}
	}
}

// Errors from HEAD, PUT and MKCOL never contain the password from a base
// URL with userinfo, whether the server answers with an error status or the
// connection fails.
func TestHTTPPublisher_RedactsUserinfo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const secret = "s3cret-pw"

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failing.Close()
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close()

	for name, raw := range map[string]string{"status": failing.URL, "connection": closedURL} {
		base := strings.Replace(raw, "://", "://user:"+secret+"@", 1) + "/registry"
		pub, err := NewHTTPPublisher(base, nil)
		if err != nil {
			t.Fatalf("%s: NewHTTPPublisher: %v", name, err)
		}

		var errs []error
		_, err = pub.Exists(ctx, "rules_go/1.0/MODULE.bazel")
		errs = append(errs, err)
		errs = append(errs, pub.Put(ctx, "rules_go/1.0/MODULE.bazel", []byte("x")))
		if name == "connection" {
			// MKCOL failures are ignored by Put; check them directly.
			errs = append(errs, pub.ensureDirectories(ctx, "rules_go/1.0/MODULE.bazel"))
		}
		for i, e := range errs {
			if e == nil {
				t.Errorf("%s: call %d returned no error", name, i)
				continue
			}
			if strings.Contains(e.Error(), secret) {
				t.Errorf("%s: call %d leaks the password: %v", name, i, e)
			}
		}
	}
}

// A base with trailing slashes and an escaped path segment produces
// requests directly under that path.
func TestHTTPPublisher_BasePathNormalisation(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var uris []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		uris = append(uris, r.Method+" "+r.RequestURI)
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	for _, tc := range []struct{ base, want string }{
		{server.URL + "/r//", "/r/rules_go/1.0/MODULE.bazel"},
		{server.URL + "/my%20reg/", "/my%20reg/rules_go/1.0/MODULE.bazel"},
		{server.URL, "/rules_go/1.0/MODULE.bazel"},
	} {
		mu.Lock()
		uris = nil
		mu.Unlock()

		pub, err := NewHTTPPublisher(tc.base, nil)
		if err != nil {
			t.Fatalf("NewHTTPPublisher(%q): %v", tc.base, err)
		}
		if err := pub.Put(context.Background(), "rules_go/1.0/MODULE.bazel", []byte("x")); err != nil {
			t.Fatalf("Put via %q: %v", tc.base, err)
		}

		mu.Lock()
		last := uris[len(uris)-1]
		for _, u := range uris {
			if strings.Contains(strings.SplitN(u, " ", 2)[1], "//") {
				t.Errorf("base %q: request with an empty segment: %s", tc.base, u)
			}
		}
		mu.Unlock()
		if last != "PUT "+tc.want {
			t.Errorf("base %q: last request = %q, want PUT %s", tc.base, last, tc.want)
		}
	}
}
