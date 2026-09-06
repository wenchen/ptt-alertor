package user

import (
	"encoding/json"

	log "github.com/Ptt-Alertor/logrus"

	"strings"

	"github.com/wenchen/ptt-alertor/connections"
	"github.com/wenchen/ptt-alertor/myutil"
	"github.com/garyburd/redigo/redis"
)

type Redis struct{}

var connectRedis = connections.Redis

const prefix string = "user:"

func (Redis) List() (accounts []string) {
	conn := connectRedis()
	defer conn.Close()
	var cursor int64
	for {
		reply, err := redis.Values(conn.Do("SCAN", cursor, "MATCH", prefix+"*", "COUNT", 200))
		if err != nil {
			log.WithField("runtime", myutil.BasicRuntimeInfo()).WithError(err).Error()
			break
		}
		if len(reply) < 2 {
			break
		}
		cursor, _ = redis.Int64(reply[0], nil)
		keys, _ := redis.Strings(reply[1], nil)
		for _, key := range keys {
			accounts = append(accounts, strings.TrimPrefix(key, prefix))
		}
		if cursor == 0 {
			break
		}
	}
	return accounts
}

func (Redis) Exist(account string) bool {
	conn := connectRedis()
	defer conn.Close()
	key := prefix + account
	bl, err := redis.Bool(conn.Do("EXISTS", key))
	if err != nil {
		log.WithField("runtime", myutil.BasicRuntimeInfo()).WithError(err).Error()
	}
	return bl
}

func (Redis) Save(account string, data interface{}) error {
	conn := connectRedis()
	defer conn.Close()
	key := prefix + account
	uJSON, err := json.Marshal(data)
	if err != nil {
		myutil.LogJSONEncode(err, data)
		return err
	}

	_, err = conn.Do("SET", key, uJSON, "NX")
	if err != nil {
		log.WithField("runtime", myutil.BasicRuntimeInfo()).WithError(err).Error()
		return err
	}
	return nil
}

func (Redis) Update(account string, user interface{}) error {
	conn := connectRedis()
	defer conn.Close()
	key := prefix + account
	uJSON, err := json.Marshal(user)
	if err != nil {
		myutil.LogJSONEncode(err, user)
		return err
	}

	_, err = conn.Do("SET", key, uJSON, "XX")
	if err != nil {
		log.WithField("runtime", myutil.BasicRuntimeInfo()).WithError(err).Error()
		return err
	}
	return nil
}

func (Redis) Find(account string, user *User) {
	conn := connectRedis()
	defer conn.Close()

	key := prefix + account
	uJSON, err := redis.Bytes(conn.Do("GET", key))
	if err != nil && err != redis.ErrNil {
		log.WithField("runtime", myutil.BasicRuntimeInfo()).WithError(err).Error()
	}

	if uJSON != nil {
		err = json.Unmarshal(uJSON, &user)
		if err != nil {
			myutil.LogJSONDecode(err, uJSON)
		}
	}
}
