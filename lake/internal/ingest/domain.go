package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

const domainAuthURL = "https://auth.domain.com.au/v1/connect/token"
const domainAPIBase = "https://api.domain.com.au/v1"

func IngestDomain(ctx context.Context, s3 *storage.Client) error {
	start := time.Now()
	source := "domain"

	clientID := os.Getenv("DOMAIN_CLIENT_ID")
	clientSecret := os.Getenv("DOMAIN_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		log.Println("domain: DOMAIN_CLIENT_ID/DOMAIN_CLIENT_SECRET not set, skipping")
		return nil
	}

	token, err := getDomainToken(ctx, clientID, clientSecret)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return fmt.Errorf("domain auth: %w", err)
	}

	var rows []map[string]interface{}

	auctions, err := fetchAuctionResults(ctx, token)
	if err != nil {
		log.Printf("domain: auction results: %v", err)
	} else {
		rows = append(rows, auctions...)
		log.Printf("domain: fetched %d auction results", len(auctions))
	}

	time.Sleep(500 * time.Millisecond)

	suburbs := []string{"Sydney", "Parramatta", "Chatswood", "Bondi"}
	for _, suburb := range suburbs {
		time.Sleep(500 * time.Millisecond)
		listings, err := fetchListings(ctx, token, suburb)
		if err != nil {
			log.Printf("domain: listings %s: %v", suburb, err)
			continue
		}
		rows = append(rows, listings...)
	}

	if len(rows) == 0 {
		log.Println("domain: no data fetched")
		return nil
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(rows, "domain", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return fmt.Errorf("write bronze: %w", err)
	}

	key, err := s3.PutParquet(ctx, "bronze", "domain", "listings.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return fmt.Errorf("put s3: %w", err)
	}

	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	log.Printf("domain: wrote %d rows to %s", len(rows), key)
	return nil
}

func getDomainToken(ctx context.Context, clientID, clientSecret string) (string, error) {
	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {"api_listings_read api_salesresults_read"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, domainAuthURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth http %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}
	return tokenResp.AccessToken, nil
}

func fetchAuctionResults(ctx context.Context, token string) ([]map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, domainAPIBase+"/salesResults/Sydney", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auction http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		var arr []interface{}
		if err2 := json.Unmarshal(body, &arr); err2 != nil {
			return nil, fmt.Errorf("unmarshal: %w", err)
		}
		var rows []map[string]interface{}
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				for _, listing := range extractListings(m) {
					listing["source"] = "auction_results"
					rows = append(rows, listing)
				}
			}
		}
		return rows, nil
	}

	var rows []map[string]interface{}
	for _, listing := range extractListings(result) {
		listing["source"] = "auction_results"
		rows = append(rows, listing)
	}
	return rows, nil
}

func extractListings(data map[string]interface{}) []map[string]interface{} {
	results, ok := data["results"].([]interface{})
	if !ok {
		return nil
	}
	auctionDate, _ := data["auctionedDate"].(string)

	var listings []map[string]interface{}
	for _, r := range results {
		m, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		geo, _ := m["geoLocation"].(map[string]interface{})
		listing := map[string]interface{}{
			"listing_id":     fmt.Sprintf("%v", m["id"]),
			"suburb":         m["suburb"],
			"state":          m["state"],
			"postcode":       m["postcode"],
			"property_type":  m["propertyType"],
			"bedrooms":       m["bedrooms"],
			"bathrooms":      m["bathrooms"],
			"price_guide":    m["price"],
			"auction_date":   auctionDate,
			"sold_price":     m["reportedPrice"],
			"days_on_market": m["daysOnMarket"],
		}
		if geo != nil {
			listing["latitude"] = geo["latitude"]
			listing["longitude"] = geo["longitude"]
		}
		listings = append(listings, listing)
	}
	return listings
}

func fetchListings(ctx context.Context, token, suburb string) ([]map[string]interface{}, error) {
	body := fmt.Sprintf(`{"listingType":"Sale","locations":[{"suburb":"%s","state":"NSW"}],"pageSize":50}`, suburb)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, domainAPIBase+"/listings/residential/_search", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listings http %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(respBody, &items); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	var rows []map[string]interface{}
	for _, item := range items {
		listing, _ := item["listing"].(map[string]interface{})
		if listing == nil {
			continue
		}
		prop, _ := listing["propertyDetails"].(map[string]interface{})
		if prop == nil {
			continue
		}
		priceDetails, _ := listing["priceDetails"].(map[string]interface{})

		row := map[string]interface{}{
			"listing_id":    fmt.Sprintf("%v", listing["id"]),
			"suburb":        prop["suburb"],
			"state":         prop["state"],
			"postcode":      prop["postcode"],
			"property_type": prop["propertyType"],
			"bedrooms":      prop["bedrooms"],
			"bathrooms":     prop["bathrooms"],
			"latitude":      prop["latitude"],
			"longitude":     prop["longitude"],
			"source":        "listings_search",
		}
		if priceDetails != nil {
			row["price_guide"] = priceDetails["displayPrice"]
		}
		rows = append(rows, row)
	}
	return rows, nil
}
