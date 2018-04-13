package main

import (
	"github.com/go-redis/redis"
	"time"
)

const textSuffix = "_TEXT"
const byteSuffix = "_BYTE"

type FileCache interface {
	SaveBytes(string, []byte) error
	GetBytes(string) ([]byte, error)
	SaveText(string, string) error
	GetText(string) (string, error)
}

type RedisCache struct {
	client  *redis.Client
	timeout int64
}

func (v *RedisCache) Connect(settings RedisSettings) error {
	client := redis.NewClient(&redis.Options{
		Addr:     settings.Url,
		Password: settings.Password,
		DB:       0,
	})
	if _, err := client.Ping().Result(); err != nil {
		return err
	}
	v.client = client
	v.timeout = settings.DefaultRecordTimeout
	return nil
}

func (v *RedisCache) Close() {
	v.client.Close()
}

func (v *RedisCache) SaveBytes(key string, value []byte) error {
	return v.client.Set(key+byteSuffix, value, time.Duration(v.timeout)).Err()
}

func (v *RedisCache) GetBytes(key string) ([]byte, error) {
	res := v.client.Get(key + byteSuffix)
	return res.Bytes()
}

func (v *RedisCache) SaveText(key string, value string) error {
	return v.client.Set(key+textSuffix, value, time.Duration(v.timeout)).Err()
}

func (v *RedisCache) GetText(key string) (string, error) {
	res := v.client.Get(key + textSuffix)
	return res.Val(), res.Err()
}
