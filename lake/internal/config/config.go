package config

import "os"

type Config struct {
	S3Endpoint   string
	S3AccessKey  string
	S3SecretKey  string
	S3Region     string
	BronzeBucket string
	SilverBucket string
	GoldBucket   string

	KafkaBrokers     string
	KafkaTopicBronze string
	KafkaTopicSilver string

	DomainClientID     string
	DomainClientSecret string

	LogLevel    string
	MetricsPort string
	HealthPort  string

	CronRBA    string
	CronABS    string
	CronAEMO   string
	CronRSS    string
	CronReddit string
	CronDomain string
	CronNSWVG  string
}

func Load() Config {
	return Config{
		S3Endpoint:   envOr("S3_ENDPOINT", "http://garage.storage.svc.cluster.local:3900"),
		S3AccessKey:  os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:  os.Getenv("S3_SECRET_KEY"),
		S3Region:     envOr("S3_REGION", "us-east-1"),
		BronzeBucket: envOr("S3_BRONZE_BUCKET", "bronze"),
		SilverBucket: envOr("S3_SILVER_BUCKET", "silver"),
		GoldBucket:   envOr("S3_GOLD_BUCKET", "gold"),

		KafkaBrokers:     envOr("KAFKA_BROKERS", "redpanda.streaming.svc.cluster.local:9093"),
		KafkaTopicBronze: envOr("KAFKA_TOPIC_BRONZE", "lake.bronze.written"),
		KafkaTopicSilver: envOr("KAFKA_TOPIC_SILVER", "lake.silver.written"),

		DomainClientID:     os.Getenv("DOMAIN_CLIENT_ID"),
		DomainClientSecret: os.Getenv("DOMAIN_CLIENT_SECRET"),

		LogLevel:    envOr("LOG_LEVEL", "info"),
		MetricsPort: envOr("METRICS_PORT", "9090"),
		HealthPort:  envOr("HEALTH_PORT", "8081"),

		CronRBA:    envOr("CRON_RBA", "0 20 * * *"),
		CronABS:    envOr("CRON_ABS", "0 21 * * *"),
		CronAEMO:   envOr("CRON_AEMO", "*/5 * * * *"),
		CronRSS:    envOr("CRON_RSS", "*/15 * * * *"),
		CronReddit: envOr("CRON_REDDIT", "*/30 * * * *"),
		CronDomain: envOr("CRON_DOMAIN", "0 23 * * *"),
		CronNSWVG:  envOr("CRON_NSW_VG", "0 0 * * 0"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
