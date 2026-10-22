package queue

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Queue 基于 Redis List 的构建任务队列：LPUSH 入队，BRPOP 出队。
type Queue struct {
	rdb *redis.Client
	key string
}

func New(addr, password string, db int, key string) (*Queue, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Queue{rdb: rdb, key: key}, nil
}

func (q *Queue) Enqueue(ctx context.Context, runID string) error {
	return q.rdb.LPush(ctx, q.key, runID).Err()
}

// Dequeue 阻塞等待任务，超时返回空字符串且不返回错误。
func (q *Queue) Dequeue(ctx context.Context, timeout time.Duration) (string, error) {
	vals, err := q.rdb.BRPop(ctx, timeout, q.key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	if len(vals) != 2 {
		return "", nil
	}
	return vals[1], nil
}

func (q *Queue) Close() error {
	return q.rdb.Close()
}
