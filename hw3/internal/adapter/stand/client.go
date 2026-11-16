package stand

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Kkaramanov/go-dev/hw3/internal/domain"
)

const maxResponseBytes = 1 << 20

type Client struct {
	baseURL string
	http    *http.Client
	timeout time.Duration
}

func New(baseURL string, client *http.Client, timeout time.Duration) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: client, timeout: timeout}
}

func (c *Client) GetMovie(ctx context.Context, id int64) (domain.Movie, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	endpoint := fmt.Sprintf("%s/%d/info.0.json", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.Movie{}, domain.Failure(id, domain.UpstreamError, "cannot create stand request", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		code := domain.Unavailable
		if ctx.Err() != nil {
			err = ctx.Err()
			code = domain.Classify(err)
		}
		return domain.Movie{}, domain.Failure(id, code, fmt.Sprintf("movie %d: stand request failed (%s)", id, code), err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		code := domain.UpstreamError
		switch resp.StatusCode {
		case http.StatusNotFound:
			code = domain.NotFound
		case http.StatusTooManyRequests:
			code = domain.RateLimited
		}
		return domain.Movie{}, domain.Failure(id, code, fmt.Sprintf("movie %d: stand returned HTTP %d", id, resp.StatusCode), nil)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		code := domain.Unavailable
		if ctx.Err() != nil {
			err = ctx.Err()
			code = domain.Classify(err)
		}
		return domain.Movie{}, domain.Failure(id, code, fmt.Sprintf("movie %d: cannot read stand response (%s)", id, code), err)
	}
	var result movieResponse
	if len(body) > maxResponseBytes {
		err = fmt.Errorf("response exceeds %d bytes", maxResponseBytes)
	} else {
		err = json.Unmarshal(body, &result)
	}
	if err != nil || result.ID != id || result.Title == "" || result.Director == "" || result.Year <= 0 {
		return domain.Movie{}, domain.Failure(id, domain.InvalidResponse, fmt.Sprintf("movie %d: invalid movie JSON from stand", id), err)
	}
	return domain.Movie{ID: result.ID, Title: result.Title, Year: result.Year, Director: result.Director}, nil
}

// movieResponse belongs to the external stand API, independently of domain models.
type movieResponse struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Year     int32  `json:"year"`
	Director string `json:"director"`
}
