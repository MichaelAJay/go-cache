Delete a Single Session

Input: sessionID

Steps:

Get subjectID from session:{sessionID}:subject

DEL session:{sessionID}

DEL session:{sessionID}:subject

SREM subject:{subjectID}:sessions sessionID

-- delete_session.lua
local sessionKey = KEYS[1]
local sessionSubjectKey = KEYS[2]
local subjectSessionsKey = KEYS[3]

local subjectID = redis.call("GET", sessionSubjectKey)
if subjectID then
redis.call("SREM", subjectSessionsKey, ARGV[1])
end

redis.call("DEL", sessionKey, sessionSubjectKey)
return 1

---

Delete All Sessions for a Subject

Input: subjectID

Steps:

Get all sessionIDs from subject:{subjectID}:sessions

For each session:

DEL session:{sessionID}

DEL session:{sessionID}:subject

DEL subject:{subjectID}:sessions

-- delete_subject_sessions.lua
local subjectSessionsKey = KEYS[1]
local sessions = redis.call("SMEMBERS", subjectSessionsKey)

for i, sessionID in ipairs(sessions) do
redis.call("DEL", "session:" .. sessionID, "session:" .. sessionID .. ":subject")
end

redis.call("DEL", subjectSessionsKey)
return #sessions

Proposed Go wrappers:
package cache

import (
"context"
"github.com/redis/go-redis/v9"
)

type Cache struct {
rdb *redis.Client
// preloaded scripts
deleteSessionScript *redis.Script
deleteSubjectSessionsScript \*redis.Script
}

func NewCache(rdb *redis.Client) *Cache {
return &Cache{
rdb: rdb,
deleteSessionScript: redis.NewScript(`
local sessionKey = KEYS[1]
local sessionSubjectKey = KEYS[2]
local subjectSessionsKey = KEYS[3]

    		local subjectID = redis.call("GET", sessionSubjectKey)
    		if subjectID then
    			redis.call("SREM", subjectSessionsKey, ARGV[1])
    		end

    		redis.call("DEL", sessionKey, sessionSubjectKey)
    		return 1
    	`),
    	deleteSubjectSessionsScript: redis.NewScript(`
    		local subjectSessionsKey = KEYS[1]
    		local sessions = redis.call("SMEMBERS", subjectSessionsKey)

    		for i, sessionID in ipairs(sessions) do
    			redis.call("DEL", "session:" .. sessionID, "session:" .. sessionID .. ":subject")
    		end

    		redis.call("DEL", subjectSessionsKey)
    		return #sessions
    	`),
    }

}

// DeleteSession removes a single session and updates its subject index.
func (c \*Cache) DeleteSession(ctx context.Context, sessionID, subjectID string) error {
keys := []string{
"session:" + sessionID,
"session:" + sessionID + ":subject",
"subject:" + subjectID + ":sessions",
}
\_, err := c.deleteSessionScript.Run(ctx, c.rdb, keys, sessionID).Result()
return err
}

// DeleteSubjectSessions removes all sessions for a subject.
func (c \*Cache) DeleteSubjectSessions(ctx context.Context, subjectID string) (int64, error) {
keys := []string{
"subject:" + subjectID + ":sessions",
}
res, err := c.deleteSubjectSessionsScript.Run(ctx, c.rdb, keys).Int64()
return res, err
}
