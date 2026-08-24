// Package websearch 提供外部网页搜索 HTTP 客户端。
package websearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
)

const maxResponseBytes = 1 << 20

// Client 调用兼容 Wikipedia OpenSearch 格式的搜索接口。
type Client struct {
	baseURL string
	timeout time.Duration
	http    *http.Client
}

// NewClient 创建网页搜索客户端。
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimSpace(baseURL),
		timeout: timeout,
		http:    &http.Client{},
	}
}

// Search 查询网页；达到超时时间后取消请求并返回空结果。
func (c *Client) Search(ctx context.Context, query string) (protocol.WebSearchResponse, error) {
	result := protocol.WebSearchResponse{
		Query:   query,
		Results: make([]protocol.WebSearchResult, 0),
	}

	requestURL, err := url.Parse(c.baseURL)
	if err != nil {
		return result, fmt.Errorf("parse web search url: %w", err)
	}
	params := requestURL.Query()
	params.Set("action", "opensearch")
	params.Set("search", query)
	params.Set("limit", "5")
	params.Set("namespace", "0")
	params.Set("format", "json")
	requestURL.RawQuery = params.Encode()

	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return result, fmt.Errorf("create web search request: %w", err)
	}
	req.Header.Set("User-Agent", "go-orm-game-demo/1.0")

	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			result.TimedOut = true
			return result, nil
		}
		return result, fmt.Errorf("request web search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return result, fmt.Errorf("web search returned status %d", resp.StatusCode)
	}

	var raw []json.RawMessage
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes))
	if err := decoder.Decode(&raw); err != nil {
		return result, fmt.Errorf("decode web search response: %w", err)
	}
	if len(raw) < 4 {
		return result, fmt.Errorf("invalid web search response")
	}

	var titles, descriptions, urls []string
	if err := json.Unmarshal(raw[1], &titles); err != nil {
		return result, fmt.Errorf("decode web search titles: %w", err)
	}
	if err := json.Unmarshal(raw[2], &descriptions); err != nil {
		return result, fmt.Errorf("decode web search descriptions: %w", err)
	}
	if err := json.Unmarshal(raw[3], &urls); err != nil {
		return result, fmt.Errorf("decode web search urls: %w", err)
	}

	for i, title := range titles {
		if i >= len(urls) {
			break
		}
		item := protocol.WebSearchResult{Title: title, URL: urls[i]}
		if i < len(descriptions) {
			item.Description = descriptions[i]
		}
		result.Results = append(result.Results, item)
	}
	return result, nil
}
