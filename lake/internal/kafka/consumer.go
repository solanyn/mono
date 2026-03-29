package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				log.Printf("kafka fetch error topic=%s partition=%d: %v", e.Topic, e.Partition, e.Err)
			}
		}
		fetches.EachRecord(func(record *kgo.Record) {
			if err := handler(ctx, record.Value); err != nil {
				log.Printf("kafka handler error: %v", err)
			}
		})
	}
}

func (c *Consumer) Close() {
	c.client.Close()
}
