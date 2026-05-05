package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Producer struct {
	client *kgo.Client
	topic  string
}

func NewProducer(brokers []string, topic string) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
		kgo.DefaultProduceTopic(topic),
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
	)
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

func (p *Producer) ProduceAsync(ctx context.Context, key, value []byte, fn func(error)) {
	record := &kgo.Record{
		Key:   key,
		Value: value,
	}
	p.client.Produce(ctx, record, func(_ *kgo.Record, err error) {
		fn(err)
	})
}

func (p *Producer) Flush(ctx context.Context) error {
	return p.client.Flush(ctx)
}

func (p *Producer) Close() {
	p.client.Close()
}
