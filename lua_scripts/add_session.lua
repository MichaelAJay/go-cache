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
