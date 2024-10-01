package cache

import (
	rediscache "github.com/go-redis/cache"
	"github.com/go-redis/redis"
	"github.com/prometheus/client_golang/prometheus"
	msgpack "gopkg.in/vmihailenco/msgpack.v2"
)

// nolint: gochecknoglobals   新建一个计数器。读数据的计数器
var cacheGets = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: "starcharts",
		Subsystem: "cache",
		Name:      "gets_total",
		Help:      "Total number of successful cache gets",
	},
)

// nolint: gochecknoglobals   新建一个计数器。存数据的计数器
var cachePuts = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: "starcharts",
		Subsystem: "cache",
		Name:      "puts_total",
		Help:      "Total number of successful cache puts",
	},
)

// nolint: gochecknoglobals  删除缓存的计数器
var cacheDeletes = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: "starcharts",
		Subsystem: "cache",
		Name:      "deletes_total",
		Help:      "Total number of successful cache deletes",
	},
)

// nolint: gochecknoinits
func init() {
	prometheus.MustRegister(cacheGets, cachePuts, cacheDeletes)
}

// Redis cache. 
type Redis struct {
	redis *redis.Client  //redis客户端
	codec *rediscache.Codec //对象序列化/反序列化成二进制数据的编解码器
}

// New redis cache.
func New(redis *redis.Client) *Redis {
	codec := &rediscache.Codec{
		Redis: redis,
		Marshal: func(v interface{}) ([]byte, error) { //将数据变成字节码，序列化
			return msgpack.Marshal(v)
		},
		Unmarshal: func(b []byte, v interface{}) error { //反序列化过程
			return msgpack.Unmarshal(b, v)
		},
	}

	return &Redis{
		redis: redis,
		codec: codec,
	}
}

// Close connections.
func (c *Redis) Close() error {
	return c.redis.Close()
}

// Get from cache by key.  从缓存中读取数据。
func (c *Redis) Get(key string, result interface{}) error {
	if err := c.codec.Get(key, result); err != nil {  //把redis里面的结果反序列化，然后放到result中
		return err
	}
	cacheGets.Inc()  //计数器+1 
	return nil
}

// Put on cache.
func (c *Redis) Put(key string, obj interface{}) error {
	if err := c.codec.Set(&rediscache.Item{ // 把值序列化并且 存到 redis中
		Key:    key,
		Object: obj,
	}); err != nil {
		return err
	}
	cachePuts.Inc()
	return nil
}

// Delete from cache.
func (c *Redis) Delete(key string) error {
	if err := c.codec.Delete(key); err != nil {
		return err
	}
	cacheDeletes.Inc()
	return nil
}
