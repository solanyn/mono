package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Consumer struct {
	client *kgo.Client
}

type Handler func(ctx context.Context, record *kgo.Record) error

func NewConsumer(brokers []string, group, topic string) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.AllowAutoTopicCreation(),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka consumer: %w", err)
	}
	return &Consumer{client: client}, nil
}

func (c *Consumer) Run(ctx context.Context, handler Handler) error {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				slog.Error("kafka fetch error", "topic", e.Topic, "partition", e.Partition, "err", e.Err)
			}
			time.Sleep(time.Second)
			continue
		}

		var committed []*kgo.Record
		fetches.EachRecord(func(record *kgo.Record) {
			if err := handler(ctx, record); err != nil {
				slog.Error("handler error", "offset", record.Offset, "err", err)
				return
			}
			committed = append(committed, record)
		})

		if len(committed) > 0 {
			if err := c.client.CommitRecords(ctx, committed...); err != nil {
				slog.Error("commit error", "err", err)
			}
		}
	}
}

func (c *Consumer) Close() {
	c.client.Close()
}
