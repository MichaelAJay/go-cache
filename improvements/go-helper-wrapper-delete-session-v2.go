package improvements

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func (c *Cache) DeleteSession(ctx context.Context, sessionID string) error {
	script := redis.NewScript(`
		local subjectID = redis.call("GET", KEYS[2])
		if not subjectID then
		  return 0
		end
		redis.call("SREM", subjectID .. ":sessions", ARGV[1])
		redis.call("DEL", KEYS[2])
		redis.call("DEL", ARGV[1])
		return 1
	`)

	keys := []string{
		"", // placeholder for subject->sessions (constructed in Lua)
		"session:" + sessionID + ":subject",
	}
	argv := []interface{}{sessionID}

	_, err := script.Run(ctx, c.redis, keys, argv...).Result()
	return err
}

func (c *Cache) DeleteSubjectSessions(ctx context.Context, subjectID string) error {
	script := redis.NewScript(`
		local sessions = redis.call("SMEMBERS", KEYS[1])
		for i, sid in ipairs(sessions) do
		  redis.call("DEL", sid)
		  redis.call("DEL", "session:" .. sid .. ":subject")
		end
		redis.call("DEL", KEYS[1])
		return #sessions
	`)

	keys := []string{"subject:" + subjectID + ":sessions"}
	argv := []interface{}{subjectID}

	_, err := script.Run(ctx, c.redis, keys, argv...).Result()
	return err
}
