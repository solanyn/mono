package ingest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const nswGovBaseURL = "https://api.onegov.nsw.gov.au"

func nswGovToken(ctx context.Context, apiKey, apiSecret string) (string, error) {
	url := fmt.Sprintf("%s/oauth/client_credential/accesstoken?grant_type=client_credentials", nswGovBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	creds := base64.StdEncoding.EncodeToString([]byte(apiKey + ":" + apiSecret))
	req.Header.Set("Authorization", "Basic "+creds)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("nsw gov auth: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nsw gov auth http %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("nsw gov auth decode: %w", err)
	}
	return tokenResp.AccessToken, nil
}

func nswGovGet(ctx context.Context, endpoint, token, apiKey string, extraHeaders map[string]string) ([]byte, error) {
	url := nswGovBaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("apikey", apiKey)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nsw gov %s http %d", endpoint, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func fuelHeaders() map[string]string {
	return map[string]string{
		"transactionid":    uuid.New().String(),
		"requesttimestamp": time.Now().UTC().Format("02/01/2006 03:04:05 PM"),
		"Content-Type":     "application/json",
	}
}
