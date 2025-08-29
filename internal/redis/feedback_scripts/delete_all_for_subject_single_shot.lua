  local subjIdxKey   = KEYS[1]
  local dataPref     = ARGV[1]
  local metaPref     = ARGV[2]
  local sessIdxPref  = ARGV[3]

  local sids = redis.call('SMEMBERS', subjIdxKey)
  local total = 0

  for i = 1, #sids do
    local sid = sids[i]
    total = total + redis.call('DEL', dataPref .. sid)
    total = total + redis.call('DEL', metaPref .. sid)
    total = total + redis.call('DEL', sessIdxPref .. sid)
  end

  total = total + redis.call('DEL', subjIdxKey)
  return total