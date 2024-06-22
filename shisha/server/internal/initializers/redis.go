package initializers

import (
	"context"

	"github.com/go-redis/redis/v8"
)

var Rdb *redis.Client

func InitRedis(Ctx context.Context, addr string) error {
	Rdb = redis.NewClient(&redis.Options{
		Addr: addr,
		// Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	_, err := Rdb.Ping(Ctx).Result()
	if err != nil {
		return err
	}
	return nil
}
