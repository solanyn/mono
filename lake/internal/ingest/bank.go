package ingest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

const bankBasePath = "/lake/bank"

// IngestBank scans /lake/bank for OFX files, parses transactions, writes to bronze, and removes processed files.
func IngestBank(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "bank"

	files, err := findOFXFiles(bankBasePath)
	if err != nil {
		return Result{}, fmt.Errorf("scan bank dir: %w", err)
	}
	if len(files) == 0 {
		return Result{Source: source, RowCount: 0}, nil
	}

	var allRows []map[string]interface{}
	var processed []string

	for _, f := range files {
		rows, err := parseOFXFile(f)
		if err != nil {
			slog.Error("bank: skipping file", "file", f, "err", err)
			continue
		}
		allRows = append(allRows, rows...)
		processed = append(processed, f)
	}

	if len(allRows) == 0 {
		return Result{Source: source, RowCount: 0}, nil
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(allRows, source, batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}

	key, err := s3.PutParquet(ctx, bucket, "bank", fmt.Sprintf("transactions-%s.parquet", time.Now().Format("20060102-150405")), data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}

	// Remove processed files
	for _, f := range processed {
		if err := os.Remove(f); err != nil {
			slog.Error("bank: failed to remove file", "file", f, "err", err)
		}
	}

	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	slog.Info("bank: wrote rows", "rows", len(allRows), "files", len(processed), "key", key)
	return Result{Source: source, Key: key, RowCount: len(allRows)}, nil
}

func findOFXFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".ofx") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func parseOFXFile(path string) ([]map[string]interface{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Derive account from path: /lake/bank/{provider}/{account}/file.ofx
	rel, _ := filepath.Rel(bankBasePath, path)
	parts := strings.Split(rel, string(filepath.Separator))
	provider := ""
	account := ""
	if len(parts) >= 3 {
		provider = parts[0]
		account = parts[1]
	}

	scanner := bufio.NewScanner(f)
	var rows []map[string]interface{}
	var current map[string]interface{}
	inTxn := false
	var bankID, acctID, acctType string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Parse account info
		if strings.HasPrefix(line, "<BANKID>") {
			bankID = stripTag(line, "BANKID")
		} else if strings.HasPrefix(line, "<ACCTID>") {
			acctID = stripTag(line, "ACCTID")
		} else if strings.HasPrefix(line, "<ACCTTYPE>") {
			acctType = stripTag(line, "ACCTTYPE")
		}

		if line == "<STMTTRN>" {
			inTxn = true
			current = map[string]interface{}{
				"provider":  provider,
				"account":   account,
				"bank_id":   bankID,
				"acct_id":   acctID,
				"acct_type": acctType,
			}
		} else if line == "</STMTTRN>" {
			if inTxn && current != nil {
				// Wrap as _raw_payload
				payload, _ := json.Marshal(current)
				rows = append(rows, map[string]interface{}{
					"_raw_payload": string(payload),
				})
			}
			inTxn = false
			current = nil
		} else if inTxn && current != nil {
			if strings.HasPrefix(line, "<TRNTYPE>") {
				current["trn_type"] = stripTag(line, "TRNTYPE")
			} else if strings.HasPrefix(line, "<DTPOSTED>") {
				current["date"] = stripTag(line, "DTPOSTED")
			} else if strings.HasPrefix(line, "<TRNAMT>") {
				current["amount"] = stripTag(line, "TRNAMT")
			} else if strings.HasPrefix(line, "<FITID>") {
				current["fit_id"] = stripTag(line, "FITID")
			} else if strings.HasPrefix(line, "<MEMO>") {
				current["memo"] = stripTag(line, "MEMO")
			} else if strings.HasPrefix(line, "<NAME>") {
				current["name"] = stripTag(line, "NAME")
			}
		}
	}

	return rows, scanner.Err()
}

func stripTag(line, tag string) string {
	prefix := "<" + tag + ">"
	return strings.TrimSpace(strings.TrimPrefix(line, prefix))
}
