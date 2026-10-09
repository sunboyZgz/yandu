package main

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeHealth struct {
	responses []*http.Response
	errors    []error
	closed    int
	index     int
}

type trackedBody struct {
	io.Reader
	closed *int
}

func (b trackedBody) Close() error { *b.closed++; return nil }
func (f *fakeHealth) Get(string) (*http.Response, error) {
	i := f.index
	f.index++
	if f.errors[i] != nil {
		return nil, f.errors[i]
	}
	r := f.responses[i]
	r.Body = trackedBody{strings.NewReader("{}"), &f.closed}
	return r, nil
}

func TestRequireRunningNeverStartsService(t *testing.T) {
	client := &fakeHealth{errors: []error{errors.New("connection refused")}}
	started := false
	err := ensureAgentRunning(client, func() error { started = true; return nil }, true)
	if err == nil || started {
		t.Fatalf("missing Agent must fail without starting anything: error=%v started=%v", err, started)
	}
}

func TestExistingAgentDoesNotStartAnotherInstance(t *testing.T) {
	client := &fakeHealth{responses: []*http.Response{{StatusCode: 200}}, errors: []error{nil}}
	started := false
	err := ensureAgentRunning(client, func() error { started = true; return nil }, true)
	if err != nil || started || client.closed != 1 {
		t.Fatalf("existing Agent: error=%v started=%v closed=%d", err, started, client.closed)
	}
}

func TestNormalLaunchRetainsAutomaticStartup(t *testing.T) {
	client := &fakeHealth{responses: []*http.Response{nil, {StatusCode: 200}}, errors: []error{errors.New("connection refused"), nil}}
	starts := 0
	err := ensureAgentRunning(client, func() error { starts++; return nil }, false)
	if err != nil || starts != 1 || client.closed != 1 {
		t.Fatalf("normal startup: error=%v starts=%d closed=%d", err, starts, client.closed)
	}
}

func TestForeignHealthResponseDoesNotTriggerServiceStartup(t *testing.T) {
	client := &fakeHealth{responses: []*http.Response{{StatusCode: 403}}, errors: []error{nil}}
	started := false
	err := ensureAgentRunning(client, func() error { started = true; return nil }, false)
	if err == nil || started || client.closed != 1 {
		t.Fatalf("foreign response: error=%v started=%v closed=%d", err, started, client.closed)
	}
}

func TestStartupFailureIsReturned(t *testing.T) {
	client := &fakeHealth{errors: []error{errors.New("connection refused")}}
	want := errors.New("service access denied")
	if err := ensureAgentRunning(client, func() error { return want }, false); !errors.Is(err, want) {
		t.Fatalf("startup error lost: %v", err)
	}
}
