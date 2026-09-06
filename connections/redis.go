package connections

import (
	"os"
	"time"

	log "github.com/Ptt-Alertor/logrus"
	"github.com/garyburd/redigo/redis"
)

var pool = newPool()

func newPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     10,
		MaxActive:   100,
		Wait:        true,
		IdleTimeout: 300 * time.Second,
		Dial: func() (redis.Conn, error) {
			endpoint := os.Getenv("REDIS_ENDPOINT")
			if endpoint == "" {
				endpoint = "localhost"
			}
			port := os.Getenv("REDIS_PORT")
			if port == "" {
				port = "6379"
			}
			var conn redis.Conn
			var err error
			for i := 0; i < 5; i++ {
				conn, err = redis.Dial("tcp", endpoint+":"+port)
				if err == nil {
					return conn, nil
				}
				time.Sleep(1 * time.Second)
			}
			log.WithError(err).Fatal("Failed to connect to Redis")
			return nil, err
		},
	}
}

// Redis get redis connection
func Redis() redis.Conn {
	return pool.Get()
}
