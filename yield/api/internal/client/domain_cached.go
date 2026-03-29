package client

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/solanyn/mono/yield/api/internal/metrics"
)

type CachedDomainClient struct {
	api   *DomainClient
	cache *DomainCache
	queue *FetchQueue
	quota int
}

func NewCachedDomainClient(api *DomainClient, cache *DomainCache, queue *FetchQueue, dailyQuota int) *CachedDomainClient {
	return &CachedDomainClient{
		api:   api,
		cache: cache,
		queue: queue,
		quota: dailyQuota,
	}
}

func (c *CachedDomainClient) Fetch(ctx context.Context, endpoint, path string, params map[string]string) ([]byte, error) {
	data, hot, err := c.cache.Get(ctx, endpoint, params)
	if err == nil && data != nil {
		if hot {
			metrics.Global.RecordCacheHit()
		} else {
			metrics.Global.RecordColdHit()
			c.queue.EnqueueStaleRefresh(ctx, endpoint+":"+path, 1)
		}
		return data, nil
	}
	metrics.Global.RecordCacheMiss()

	used, _ := c.queue.QuotaUsed(ctx)
	reserve := int64(50)
	if used >= int64(c.quota)-reserve {
		c.queue.EnqueueUserRequest(ctx, endpoint+":"+path)
		return nil, fmt.Errorf("domain quota exhausted (%d/%d), request queued", used, c.quota)
	}

	data, err = c.api.Get(ctx, path)
	if err != nil {
		c.queue.EnqueueUserRequest(ctx, endpoint+":"+path)
		return nil, fmt.Errorf("domain api: %w", err)
	}

	c.queue.IncrementQuota(ctx)
	metrics.Global.RecordDomainReq()

	if putErr := c.cache.Put(ctx, endpoint, params, data); putErr != nil {
		log.Printf("domain cache put: %v", putErr)
	}

	return data, nil
}

func (c *CachedDomainClient) SearchRentalListings(ctx context.Context, suburb, state string, bedrooms int, pageSize int) ([]byte, error) {
	params := map[string]string{
		"suburb":   suburb,
		"state":    state,
		"bedrooms": fmt.Sprintf("%d", bedrooms),
	}

	path := "/v1/listings/residential/_search"
	cacheData, hot, _ := c.cache.Get(ctx, "listings_search", params)
	if cacheData != nil {
		if hot {
			metrics.Global.RecordCacheHit()
		} else {
			metrics.Global.RecordColdHit()
		}
		return cacheData, nil
	}
	metrics.Global.RecordCacheMiss()

	used, _ := c.queue.QuotaUsed(ctx)
	if used >= int64(c.quota)-50 {
		c.queue.EnqueueUserRequest(ctx, "listings_search:"+suburb)
		return nil, fmt.Errorf("domain quota exhausted, request queued")
	}

	body := fmt.Sprintf(`{
		"listingType": "Rent",
		"locations": [{"suburb": "%s", "state": "%s"}],
		"minBedrooms": %d,
		"maxBedrooms": %d,
		"pageSize": %d
	}`, suburb, state, bedrooms, bedrooms, pageSize)

	token, err := c.api.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("domain auth: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.domain.com.au"+path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.api.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("domain listings search: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	c.queue.IncrementQuota(ctx)
	metrics.Global.RecordDomainReq()
	c.cache.Put(ctx, "listings_search", params, data)

	return data, nil
}

func (c *CachedDomainClient) GetSuburbPerformance(ctx context.Context, state string, suburbID int, propertyCategory string, years int) ([]byte, error) {
	params := map[string]string{
		"state":            state,
		"suburbId":         fmt.Sprintf("%d", suburbID),
		"propertyCategory": propertyCategory,
		"years":            fmt.Sprintf("%d", years),
	}

	path := fmt.Sprintf("/v1/suburbPerformanceStatistics?state=%s&suburbId=%d&propertyCategory=%s&chronologicalSpan=12&tPlusFrom=1&tPlusTo=%d",
		state, suburbID, propertyCategory, years)

	return c.Fetch(ctx, "suburb_stats", path, params)
}

func (c *CachedDomainClient) GetProperty(ctx context.Context, propertyID string) ([]byte, error) {
	params := map[string]string{"propertyId": propertyID}
	path := fmt.Sprintf("/v1/properties/%s", propertyID)
	return c.Fetch(ctx, "property_profile", path, params)
}

func (c *CachedDomainClient) SuggestProperties(ctx context.Context, terms string) ([]byte, error) {
	params := map[string]string{"terms": terms}
	path := fmt.Sprintf("/v1/properties/_suggest?terms=%s", terms)
	return c.Fetch(ctx, "property_suggest", path, params)
}

func (c *CachedDomainClient) DrainQueue(ctx context.Context, maxRequests int) (int, error) {
	drained := 0
	items, err := c.queue.Pop(ctx, int64(maxRequests))
	if err != nil {
		return 0, err
	}

	for _, item := range items {
		parts := strings.SplitN(item, ":", 2)
		if len(parts) != 2 {
			continue
		}
		endpoint, path := parts[0], parts[1]

		used, _ := c.queue.QuotaUsed(ctx)
		if used >= int64(c.quota)-50 {
			c.queue.Enqueue(ctx, item, 50)
			break
		}

		data, err := c.api.Get(ctx, path)
		if err != nil {
			log.Printf("domain drain %s: %v", item, err)
			continue
		}

		c.queue.IncrementQuota(ctx)
		metrics.Global.RecordDomainReq()
		metrics.Global.RecordQueueDrain()

		params := map[string]string{"path": path}
		c.cache.Put(ctx, endpoint, params, data)
		drained++
	}

	return drained, nil
}
