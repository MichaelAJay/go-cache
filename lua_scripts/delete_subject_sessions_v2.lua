-- KEYS[1] = subjectID -> sessions set
-- ARGV[1] = subjectID

local sessions = redis.call("SMEMBERS", KEYS[1])

for i, sid in ipairs(sessions) do
  -- Delete session payload
  redis.call("DEL", sid)

  -- Delete reverse index
  redis.call("DEL", "session:" .. sid .. ":subject")
end

-- Finally, delete the set itself
redis.call("DEL", KEYS[1])

return #sessions
