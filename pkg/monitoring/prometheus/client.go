package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/combi/pkg/log"
)

// Adapter defines the contract for querying metrics.
type Adapter interface {
	// QueryScalar executes a PromQL query that returns a single value (vector).
	// Returns true if the query returned data satisfying the condition.
	QueryCondition(ctx context.Context, query string) (bool, float64, error)
}

type client struct {
	baseURL    string
	httpClient *http.Client
	logger     log.Logger
}

func NewClient(baseURL string, logger log.Logger) Adapter {
	return &client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

// PromResponse is a partial schema for the Prometheus API response.
type PromResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			// Value is [timestamp, string_value]
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func (c *client) QueryCondition(ctx context.Context, query string) (bool, float64, error) {
	u, err := url.Parse(fmt.Sprintf("%s/api/v1/query", c.baseURL))
	if err != nil {
		return false, 0, err
	}
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return false, 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, 0, fmt.Errorf("prometheus request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, 0, fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}

	var pr PromResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return false, 0, fmt.Errorf("failed to decode prometheus response: %w", err)
	}

	if pr.Status != "success" {
		return false, 0, fmt.Errorf("prometheus query error")
	}

	// Logic: If we get any result in the vector, it means the condition matches.
	// Prometheus "Instant Queries" for alerting usually return the elements that MATCH the condition.
	// Example: `up == 0` returns only instances that are down.
	if len(pr.Data.Result) > 0 {
		// Parse the first value for logging/threshold purposes
		valRaw := pr.Data.Result[0].Value
		if len(valRaw) == 2 {
			if s, ok := valRaw[1].(string); ok {
				f, _ := strconv.ParseFloat(s, 64)
				return true, f, nil
			}
		}
		return true, 0, nil
	}

	return false, 0, nil
}
