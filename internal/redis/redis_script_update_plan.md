Redis Cache Delete Handling Plan

1. Core Problem

Current delete logic uses KEYS queries (redis.call("KEYS", ...)) → not scalable in production, risk of blocking the server.

Indexes for relationships (e.g., subjectID → sessionIDs) are currently held outside Redis, creating consistency problems when multiple processes interact.

Need bi-directional indexes for efficient deletes:

subjectID → sessionIDs (all sessions for a subject).

sessionID → subjectID (find subject when deleting a session).

2. Corrected Design

Store all indexes inside Redis (no in-process only state).

Use Redis Sets:

subject:{subjectID}:sessions → set of sessionIDs for a subject.

session:{sessionID}:subject → single subjectID for a session.

Deletes use these indexes:

DeleteSession(sessionID):

Lookup subjectID via session:{sessionID}:subject.

Remove sessionID from subject:{subjectID}:sessions.

Delete the session key itself.

Delete session:{sessionID}:subject.

DeleteSubjectSessions(subjectID):

Lookup all sessionIDs via subject:{subjectID}:sessions.

Delete all session keys + session:{sessionID}:subject entries.

Delete subject:{subjectID}:sessions.

DeleteSession.lua
-- KEYS[1] = session:{sessionID}:subject
-- KEYS[2] = subject:{subjectID}:sessions
-- KEYS[3] = session:{sessionID} (actual session data)

local subjectID = redis.call("GET", KEYS[1])
if not subjectID then
return 0
end

redis.call("SREM", "subject:" .. subjectID .. ":sessions", ARGV[1])
redis.call("DEL", KEYS[1], KEYS[3])
return 1

DeleteSubjectSessions.lua
-- KEYS[1] = subject:{subjectID}:sessions

local sessions = redis.call("SMEMBERS", KEYS[1])
for \_, sid in ipairs(sessions) do
redis.call("DEL", "session:" .. sid, "session:" .. sid .. ":subject")
end
redis.call("DEL", KEYS[1])
return #sessions

Go Helper Wrappers
package cache

import (
"context"
"github.com/redis/go-redis/v9"
)

type SessionCache struct {
rdb \*redis.Client
ctx context.Context
}

// NewSessionCache constructor
func NewSessionCache(rdb *redis.Client) *SessionCache {
return &SessionCache{
rdb: rdb,
ctx: context.Background(),
}
}

// Delete a single session
func (c \*SessionCache) DeleteSession(sessionID string) (bool, error) {
keys := []string{
"session:" + sessionID + ":subject", // KEYS[1]
"", // placeholder (subject set resolved in script)
"session:" + sessionID, // KEYS[3]
}
return c.rdb.EvalSha(c.ctx, deleteSessionSha, keys, sessionID).Bool()
}

// Delete all sessions for a subject
func (c \*SessionCache) DeleteSubjectSessions(subjectID string) (int64, error) {
keys := []string{
"subject:" + subjectID + ":sessions", // KEYS[1]
}
return c.rdb.EvalSha(c.ctx, deleteSubjectSessionsSha, keys).Int64()
}
