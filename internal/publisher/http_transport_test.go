package publisher

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHTTPPublisher_OwnsItsTransport pins the root cause of an intermittent
// "http: CloseIdleConnections called" failure. A publisher left on
// http.DefaultTransport shares its connection pool with the whole process, so
// anything that flushes that pool (httptest.Server.Close in a parallel test,
// or any other caller) can fail its in-flight requests. Each publisher must
// therefore have a private transport, which also keeps the default
// transport's settings.
func TestHTTPPublisher_OwnsItsTransport(t *testing.T) {
	t.Parallel()
	a, err := NewHTTPPublisher("https://example.com", nil)
	if err != nil {
		t.Fatalf("NewHTTPPublisher(a) error = %v", err)
	}
	defer a.Close()
	b, err := NewHTTPPublisher("https://example.com", nil)
	if err != nil {
		t.Fatalf("NewHTTPPublisher(b) error = %v", err)
	}
	defer b.Close()

	for name, p := range map[string]*HTTPPublisher{"a": a, "b": b} {
		tr, ok := p.httpClient.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("publisher %s transport is %T, want *http.Transport", name, p.httpClient.Transport)
		}
		if tr == http.DefaultTransport {
			t.Fatalf("publisher %s uses the process-wide http.DefaultTransport", name)
		}
		if tr != p.transport {
			t.Fatalf("publisher %s: Close would release a different transport than the client uses", name)
		}
	}
	if a.transport == b.transport {
		t.Fatal("two publishers share one transport; closing one would disturb the other")
	}

	def := http.DefaultTransport.(*http.Transport)
	if a.transport.Proxy == nil && def.Proxy != nil {
		t.Error("cloned transport lost the default proxy settings")
	}
	if a.httpClient.Timeout != httpTimeout {
		t.Errorf("client timeout = %v, want %v", a.httpClient.Timeout, httpTimeout)
	}
	if a.httpClient.CheckRedirect != nil {
		t.Error("redirect policy changed; it must stay the net/http default")
	}
}

// failingRoundTripper stands in for an instrumented default transport.
type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("the replaced default transport must not be used")
}

// TestHTTPPublisher_NonStandardDefaultTransport checks that a replaced
// http.DefaultTransport does not make NewHTTPPublisher panic and that the
// publisher still works.
//
// It mutates a global, so it deliberately does not call t.Parallel(). That is
// safe: the testing package runs sequential tests to completion before it
// resumes any parallel test (which pauses at its t.Parallel() call), so no
// other test in this package can observe the swap.
func TestHTTPPublisher_NonStandardDefaultTransport(t *testing.T) {
	orig := http.DefaultTransport
	http.DefaultTransport = failingRoundTripper{}
	t.Cleanup(func() { http.DefaultTransport = orig })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	pub, err := NewHTTPPublisher(srv.URL, nil)
	if err != nil {
		t.Fatalf("NewHTTPPublisher() error = %v", err)
	}
	defer pub.Close()
	if err := pub.Put(context.Background(), "m/a.json", []byte("x")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
}
