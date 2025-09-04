-- KEYS[1] = subjectID -> sessions set
-- KEYS[2] = sessionID -> subject key
-- ARGV[1] = sessionID

-- Get subjectID from the reverse index
local subjectID = redis.call("GET", KEYS[2])
if not subjectID then
  return 0 -- session not found
end

-- Remove sessionID from subject->sessions set
redis.call("SREM", subjectID .. ":sessions", ARGV[1])

-- Delete reverse mapping
redis.call("DEL", KEYS[2])

-- Delete session payload itself
redis.call("DEL", ARGV[1])

return 1
