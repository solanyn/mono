package kafka

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Producer struct {
	client *kgo.Client
	topic  string
}

func NewProducer(brokers []string, opts ...string) (*Producer, error) {
	var topic string
	if len(opts) > 0 {
		topic = opts[0]
	}

	kopts := []kgo.Opt{
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
	}
	if topic != "" {
		kopts = append(kopts, kgo.DefaultProduceTopic(topic))
	}

	client, err := kgo.NewClient(kopts...)
	if err != nil {
		return nil, fmt.Errorf("kafka producer: %w", err)
	}
	return &Producer{client: client, topic: topic}, nil
}

func (p *Producer) Produce(ctx context.Context, key, value []byte) error {
	record := &kgo.Record{
		Key:   key,
		Value: value,
	}
	results := p.client.ProduceSync(ctx, record)
	if err := results.FirstErr(); err != nil {
		return fmt.Errorf("kafka produce: %w", err)
	}
	return nil
}

func (p *Producer) ProduceAsync(ctx context.Context, topic string, key string, value []byte) {
	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	}
	p.client.Produce(ctx, record, func(_ *kgo.Record, err error) {
		if err != nil {
			slog.Error("kafka produce async failed", "topic", topic, "key", key, "err", err)
		}
	})
}

func (p *Producer) Flush(ctx context.Context) error {
	return p.client.Flush(ctx)
}

func (p *Producer) Close() {
	p.client.Close()
}
