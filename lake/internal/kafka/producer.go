package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Producer struct {
	client *kgo.Client
}

func NewProducer(brokers []string) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka producer: %w", err)
	}
	return &Producer{client: client}, nil
}

func (p *Producer) PublishBronzeWritten(ctx context.Context, event BronzeWritten) error {
	return p.publish(ctx, "lake.bronze.written", event.Source, event)
}

func (p *Producer) PublishSilverWritten(ctx context.Context, event SilverWritten) error {
	return p.publish(ctx, "lake.silver.written", event.Source, event)
}

func (p *Producer) publish(ctx context.Context, topic, key string, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: data,
	}

	results := p.client.ProduceSync(ctx, record)
	if err := results.FirstErr(); err != nil {
		return fmt.Errorf("kafka produce %s: %w", topic, err)
	}

	log.Printf("kafka: published to %s key=%s", topic, key)
	return nil
}

func (p *Producer) Close() {
	p.client.Close()
}
