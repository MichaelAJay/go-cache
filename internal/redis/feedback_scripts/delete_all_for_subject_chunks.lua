  local subjIdxKey  = KEYS[1]
  local dataPref    = ARGV[1]
  local metaPref    = ARGV[2]
  local sessIdxPref = ARGV[3]
  local limit       = tonumber(ARGV[4]) or 1000

  local count = 0

  while count < limit do
    local sid = redis.call('SPOP', subjIdxKey)
    if not sid then
      break
    end

    redis.call('DEL', dataPref .. sid)
    redis.call('DEL', metaPref .. sid)
    redis.call('DEL', sessIdxPref .. sid)

    count = count + 1
  end

  -- If set is empty now, clean it up (SPOP already empties it eventually; DEL is cheap)
  if redis.call('SCARD', subjIdxKey) == 0 then
    redis.call('DEL', subjIdxKey)
  end

  return count  -- number of sessions deleted this round