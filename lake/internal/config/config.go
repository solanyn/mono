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

	CronASX       string
	CronABSBA     string
	CronABSMig    string
	CronRBACredit string
	CronWeather   string
	CronGitHub    string
	CronPyPI      string
	CronNpm       string
	CronHN        string

	CronNSWFuel     string
	CronNSWProperty string
	CronNSWTrades   string

	CronVicVG string
	CronSQM   string
	CronBank  string

	IcebergCatalogURI string
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

		CronASX:       envOr("CRON_ASX", "0 * * * *"),
		CronABSBA:     envOr("CRON_ABS_BA", "0 22 * * *"),
		CronABSMig:    envOr("CRON_ABS_MIG", "30 22 * * *"),
		CronRBACredit: envOr("CRON_RBA_CREDIT", "30 20 * * *"),
		CronWeather:   envOr("CRON_WEATHER", "0 6 * * *"),
		CronGitHub:    envOr("CRON_GITHUB", "0 12 * * *"),
		CronPyPI:      envOr("CRON_PYPI", "0 13 * * *"),
		CronNpm:       envOr("CRON_NPM", "30 13 * * *"),
		CronHN:        envOr("CRON_HN", "0 * * * *"),

		CronNSWFuel:     envOr("CRON_NSW_FUEL", "0 * * * *"),
		CronNSWProperty: envOr("CRON_NSW_PROPERTY", "0 3 * * *"),
		CronNSWTrades:   envOr("CRON_NSW_TRADES", "30 3 * * *"),

		CronVicVG: envOr("CRON_VIC_VG", "0 2 * * 0"),   // Weekly Sunday 2am
		CronSQM:   envOr("CRON_SQM", "0 4 * * 1"),       // Weekly Monday 4am
		CronBank: envOr("CRON_BANK", "*/5 * * * *"),  // Every 5 min - check for new OFX files

		IcebergCatalogURI: envOr("ICEBERG_CATALOG_URI", "http://lakekeeper.storage.svc.cluster.local:8181/catalog"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
