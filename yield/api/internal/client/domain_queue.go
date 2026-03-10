package client

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type FetchQueue struct {
	rdb *redis.Client
}

func NewFetchQueue(rdb *redis.Client) *FetchQueue {
	return &FetchQueue{rdb: rdb}
}

func (q *FetchQueue) Enqueue(ctx context.Context, item string, priority float64) error {
	return q.rdb.ZAdd(ctx, "domain:fetch_queue", redis.Z{
		Score:  priority,
		Member: item,
	}).Err()
}

func (q *FetchQueue) EnqueueUserRequest(ctx context.Context, item string) error {
	return q.Enqueue(ctx, item, 100)
}

func (q *FetchQueue) EnqueueStaleRefresh(ctx context.Context, item string, daysSinceRefresh int) error {
	return q.Enqueue(ctx, item, 50+float64(daysSinceRefresh))
}

func (q *FetchQueue) EnqueueDiscovery(ctx context.Context, item string) error {
	return q.Enqueue(ctx, item, 10)
}

func (q *FetchQueue) Pop(ctx context.Context, count int64) ([]string, error) {
	results, err := q.rdb.ZRevRangeByScore(ctx, "domain:fetch_queue", &redis.ZRangeBy{
		Min:   "-inf",
		Max:   "+inf",
		Count: count,
	}).Result()
	if err != nil {
		return nil, err
	}

	if len(results) > 0 {
		members := make([]interface{}, len(results))
		for i, r := range results {
			members[i] = r
		}
		q.rdb.ZRem(ctx, "domain:fetch_queue", members...)
	}

	return results, nil
}

func (q *FetchQueue) Depth(ctx context.Context) (int64, error) {
	return q.rdb.ZCard(ctx, "domain:fetch_queue").Result()
}

func (q *FetchQueue) IncrementQuota(ctx context.Context) (int64, error) {
	key := fmt.Sprintf("domain:quota:%s", time.Now().Format("2006-01-02"))
	count, err := q.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	q.rdb.ExpireAt(ctx, key, quotaResetTime())
	return count, nil
}

func (q *FetchQueue) QuotaUsed(ctx context.Context) (int64, error) {
	key := fmt.Sprintf("domain:quota:%s", time.Now().Format("2006-01-02"))
	val, err := q.rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

func quotaResetTime() time.Time {
	loc, _ := time.LoadLocation("Australia/Sydney")
	now := time.Now().In(loc)
	reset := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, loc)
	if now.After(reset) {
		reset = reset.Add(24 * time.Hour)
	}
	return reset
}
