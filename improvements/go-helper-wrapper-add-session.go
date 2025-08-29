package improvements

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type SessionCache struct {
	client    *redis.Client
	addScript *redis.Script
}

func NewSessionCache(client *redis.Client) *SessionCache {
	return &SessionCache{
		client: client,
		addScript: redis.NewScript(`
			-- ARGV[1] = subjectID
			-- ARGV[2] = sessionID
			-- ARGV[3] = TTL in seconds (optional, 0 means no TTL)

			local subjectID = ARGV[1]
			local sessionID = ARGV[2]
			local ttl = tonumber(ARGV[3])

			-- Keys
			local subjectSessionsKey = "subject:" .. subjectID .. ":sessions"
			local sessionSubjectKey = "session:" .. sessionID .. ":subject"

			-- 1. Add sessionID to subject's sessions set
			redis.call("SADD", subjectSessionsKey, sessionID)

			-- 2. Map sessionID -> subjectID
			redis.call("SET", sessionSubjectKey, subjectID)

			-- 3. Apply TTL if requested
			if ttl > 0 then
			  redis.call("EXPIRE", subjectSessionsKey, ttl)
			  redis.call("EXPIRE", sessionSubjectKey, ttl)
			end

			return 1
		`),
	}
}

// AddSession stores a session and maintains indexes.
// ttl=0 means no expiration.
func (s *SessionCache) AddSession(ctx context.Context, subjectID, sessionID string, ttl time.Duration) error {
	ttlSeconds := int64(ttl.Seconds())
	if ttl < 0 {
		ttlSeconds = 0
	}

	_, err := s.addScript.Run(ctx, s.client, []string{}, subjectID, sessionID, ttlSeconds).Result()
	return err
}
