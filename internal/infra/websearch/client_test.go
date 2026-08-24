package websearch

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSearchReturnsWikipediaResults(t *testing.T) {
	client := NewClient("https://search.example/api", time.Second)
	client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Query().Get("search"); got != "游戏服务器" {
			t.Errorf("search query = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`["游戏服务器",["结果一"],["描述一"],["https://example.com/1"]]`)),
			Header:     make(http.Header),
		}, nil
	})

	result, err := client.Search(context.Background(), "游戏服务器")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if result.TimedOut {
		t.Fatal("TimedOut = true, want false")
	}
	if len(result.Results) != 1 || result.Results[0].Title != "结果一" {
		t.Fatalf("results = %#v", result.Results)
	}
}

func TestSearchTimeoutDiscardsResult(t *testing.T) {
	client := NewClient("https://search.example/api", 20*time.Millisecond)
	client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})

	result, err := client.Search(context.Background(), "slow")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !result.TimedOut {
		t.Fatal("TimedOut = false, want true")
	}
	if len(result.Results) != 0 {
		t.Fatalf("results = %#v, want empty", result.Results)
	}
}
