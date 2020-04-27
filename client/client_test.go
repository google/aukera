package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalServiceServerRunning(t *testing.T) {
	tests := []struct {
		in  func(http.ResponseWriter, *http.Request)
		out bool
	}{{
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("forbidden"))
		}, false}, {
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "missing", 404)
		}, false}, {
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/status" {
				fmt.Fprintln(w, "ok")
			} else {
				http.Error(w, "missing", 404)
			}
		}, true},
	}
	for i, tt := range tests {
		ts := httptest.NewServer(http.HandlerFunc(tt.in))
		s := Test(ts.URL)
		if s != tt.out {
			t.Errorf("Test(%v) = %v, want %v", i, s, tt.out)
		}
	}
}

func TestLocalServiceServerStopped(t *testing.T) {
	tests := []struct {
		in  func(http.ResponseWriter, *http.Request)
		out bool
	}{{
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/status" {
				fmt.Fprintln(w, "Hello, client")
			}
		}, false},
	}
	for _, tt := range tests {
		ts := httptest.NewUnstartedServer(http.HandlerFunc(tt.in))
		s := Test(ts.URL)
		if s != tt.out {
			t.Errorf("Test(%v) = %v, want %v", ts.URL, s, tt.out)
		}
	}
}
