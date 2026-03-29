package kafka

import "time"

type BronzeWritten struct {
	Source    string    `json:"source"`
	Bucket    string    `json:"bucket"`
	Key       string    `json:"key"`
	Timestamp time.Time `json:"timestamp"`
	RowCount  int       `json:"row_count"`
}

type SilverWritten struct {
	Source    string    `json:"source"`
	Bucket    string    `json:"bucket"`
	Key       string    `json:"key"`
	Timestamp time.Time `json:"timestamp"`
	RowCount  int       `json:"row_count"`
}
