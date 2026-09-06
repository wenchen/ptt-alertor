package article

import (
	"strings"

	log "github.com/Ptt-Alertor/logrus"
	"github.com/wenchen/ptt-alertor/connections"
	"github.com/wenchen/ptt-alertor/myutil"
	"github.com/garyburd/redigo/redis"
)

type Articles []Article

func (as Articles) List() []string {
	conn := connections.Redis()
	defer conn.Close()
	codes, err := redis.Strings(conn.Do("SMEMBERS", subsCodesKey))
	if err != nil && err != redis.ErrNil {
		log.WithField("runtime", myutil.BasicRuntimeInfo()).WithError(err).Error("Get subsCodesKey Failed")
	}
	if err == nil && len(codes) > 0 {
		return codes
	}

	scannedCodes := scanArticleSubs(conn)
	if len(scannedCodes) > 0 {
		args := redis.Args{}.Add(subsCodesKey).AddFlat(scannedCodes)
		_, _ = conn.Do("SADD", args...)
		return scannedCodes
	}
	return codes
}

func scanArticleSubs(conn redis.Conn) []string {
	var cursor int64
	var codes []string
	pattern := prefix + "*" + subsSuffix
	for {
		reply, err := redis.Values(conn.Do("SCAN", cursor, "MATCH", pattern, "COUNT", 100))
		if err != nil || len(reply) < 2 {
			break
		}
		cursor, _ = redis.Int64(reply[0], nil)
		keys, _ := redis.Strings(reply[1], nil)
		for _, key := range keys {
			code := strings.TrimSuffix(strings.TrimPrefix(key, prefix), subsSuffix)
			codes = append(codes, code)
		}
		if cursor == 0 {
			break
		}
	}
	return codes
}

func (as Articles) String() string {
	var content string
	for _, a := range as {
		content += "\r\n\r\n" + a.String()
	}
	return content
}

func (as Articles) StringWithPushSum() string {
	var content string
	for _, a := range as {
		content += "\r\n\r\n" + a.StringWithPushSum()
	}
	return content
}
