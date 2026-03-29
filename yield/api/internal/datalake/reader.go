package datalake

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"
)

type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Region    string
	Bucket    string
}

type DomainListing struct {
	ListingID    string   `json:"listing_id"`
	Suburb       string   `json:"suburb"`
	State        string   `json:"state"`
	Postcode     string   `json:"postcode"`
	PropertyType string   `json:"property_type"`
	Bedrooms     *int32   `json:"bedrooms"`
	Bathrooms    *int32   `json:"bathrooms"`
	PriceGuide   string   `json:"price_guide"`
	AuctionDate  string   `json:"auction_date"`
	SoldPrice    *float64 `json:"sold_price"`
	DaysOnMarket *int32   `json:"days_on_market"`
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`
}

type Reader struct {
	cfg    S3Config
	client *http.Client
}

func NewReader(cfg S3Config) *Reader {
	return &Reader{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *Reader) ReadDomainListings(ctx context.Context) ([]byte, error) {
	return r.readLatestSilver(ctx, "domain_listings", "domain_listings.parquet")
}

func (r *Reader) ReadNSWVGSales(ctx context.Context) ([]byte, error) {
	return r.readLatestSilver(ctx, "nsw_vg_sales", "nsw_vg_sales.parquet")
}

func (r *Reader) readLatestSilver(ctx context.Context, dataset, filename string) ([]byte, error) {
	now := time.Now().UTC()
	for i := 0; i < 7; i++ {
		d := now.AddDate(0, 0, -i)
		key := fmt.Sprintf("silver/%s/%04d/%02d/%02d/%s", dataset, d.Year(), d.Month(), d.Day(), filename)
		data, err := r.getObject(ctx, key)
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("no silver %s data found in last 7 days", dataset)
}

func (r *Reader) getObject(ctx context.Context, key string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/%s", r.cfg.Endpoint, r.cfg.Bucket, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	r.signRequest(req, key)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("not found: %s", key)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("s3 get %s: %d", key, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (r *Reader) signRequest(req *http.Request, key string) {
	now := time.Now().UTC()
	datestamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", "UNSIGNED-PAYLOAD")

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:UNSIGNED-PAYLOAD\nx-amz-date:%s\n",
		req.Host, amzDate)

	canonicalRequest := strings.Join([]string{
		req.Method,
		"/" + path.Join(r.cfg.Bucket, key),
		"",
		canonicalHeaders,
		signedHeaders,
		"UNSIGNED-PAYLOAD",
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", datestamp, r.cfg.Region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate, credentialScope, sha256Hex([]byte(canonicalRequest)))

	signingKey := getSignatureKey(r.cfg.SecretKey, datestamp, r.cfg.Region, "s3")
	signature := hmacSHA256Hex(signingKey, []byte(stringToSign))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		r.cfg.AccessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

func (r *Reader) ListDomainListingsBySuburb(ctx context.Context, suburb string) ([]DomainListing, error) {
	data, err := r.ReadDomainListings(ctx)
	if err != nil {
		return nil, err
	}

	listings, err := readDomainParquet(data)
	if err != nil {
		return nil, err
	}

	upper := strings.ToUpper(suburb)
	var filtered []DomainListing
	for _, l := range listings {
		if strings.ToUpper(l.Suburb) == upper {
			filtered = append(filtered, l)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].SoldPrice != nil && filtered[j].SoldPrice != nil {
			return *filtered[i].SoldPrice > *filtered[j].SoldPrice
		}
		return filtered[i].SoldPrice != nil
	})

	return filtered, nil
}

func readDomainParquet(data []byte) ([]DomainListing, error) {
	reader := bytes.NewReader(data)
	return readParquetListings(reader)
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
