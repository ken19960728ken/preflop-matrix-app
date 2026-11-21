package gui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serveGuarded(method, target string, header map[string]string) (*httptest.ResponseRecorder, bool) {
	reached := false
	h := guardRuntime(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	req := httptest.NewRequest(method, target, strings.NewReader(""))
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec, reached
}

func TestGuardRuntimeRejectsNonPost(t *testing.T) {
	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions} {
		rec, reached := serveGuarded(m, "/wails/runtime?object=0&method=0", nil)
		if rec.Code != http.StatusMethodNotAllowed || reached || rec.Header().Get("Allow") != "POST" {
			t.Errorf("%s: status %d, reached %v, Allow %q", m, rec.Code, reached, rec.Header().Get("Allow"))
		}
	}
}

func TestGuardRuntimeRejectsOpaqueOrigin(t *testing.T) {
	rec, reached := serveGuarded(http.MethodPost, "/wails/runtime", map[string]string{"Origin": "null"})
	if rec.Code != http.StatusForbidden || reached {
		t.Errorf("status %d, reached %v", rec.Code, reached)
	}
}

func TestGuardRuntimeLetsFrontendThrough(t *testing.T) {
	cases := []struct {
		method, target string
		header         map[string]string
	}{
		{http.MethodPost, "/wails/runtime", nil},
		{http.MethodPost, "/wails/runtime", map[string]string{"Origin": "wails://wails"}},
		{http.MethodGet, "/wails/runtime.js", nil},
		{http.MethodGet, "/wails/stream/poll", nil},
		{http.MethodGet, "/", nil},
		{http.MethodGet, "/report/current.html", nil},
	}
	for _, c := range cases {
		if rec, reached := serveGuarded(c.method, c.target, c.header); !reached || rec.Code != http.StatusOK {
			t.Errorf("%s %s blocked (%d)", c.method, c.target, rec.Code)
		}
	}
}
