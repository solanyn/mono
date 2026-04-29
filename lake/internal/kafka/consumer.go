package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Consumer struct {
	client *kgo.Client
}

func NewConsumer(brokers []string, group, topic string) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.AllowAutoTopicCreation(),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka consumer: %w", err)
	}
	return &Consumer{client: client}, nil
}

func (c *Consumer) ConsumeBronzeWritten(ctx context.Context, handler func(context.Context, BronzeWritten) error) error {
	return c.consume(ctx, func(ctx context.Context, data []byte) error {
		var event BronzeWritten
		if err := json.Unmarshal(data, &event); err != nil {
			return fmt.Errorf("unmarshal bronze event: %w", err)
		}
		return handler(ctx, event)
	})
}

func (c *Consumer) ConsumeSilverWritten(ctx context.Context, handler func(context.Context, SilverWritten) error) error {
	return c.consume(ctx, func(ctx context.Context, data []byte) error {
		var event SilverWritten
		if err := json.Unmarshal(data, &event); err != nil {
			return fmt.Errorf("unmarshal silver event: %w", err)
		}
		return handler(ctx, event)
	})
}

func (c *Consumer) consume(ctx context.Context, handler func(context.Context, []byte) error) error {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				if errors.Is(e.Err, context.Canceled) {
					return ctx.Err()
				}
				slog.Error("kafka fetch error", "topic", e.Topic, "partition", e.Partition, "err", e.Err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			if backoff *= 2; backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = time.Second

		var successful []*kgo.Record
		fetches.EachRecord(func(record *kgo.Record) {
			if err := handler(ctx, record.Value); err != nil {
				slog.Error("kafka handler error", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", err)
				return
			}
			successful = append(successful, record)
		})

		if len(successful) == 0 {
			continue
		}
		if err := c.client.CommitRecords(ctx, successful...); err != nil {
			slog.Error("kafka commit error", "err", err)
		}
	}
}

func (c *Consumer) Close() {
	c.client.Close()
}
