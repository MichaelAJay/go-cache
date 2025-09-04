  local dataKey      = KEYS[1]
  local metaKey      = KEYS[2]
  local sessionIdx   = KEYS[3]
  local subjIdxPref  = ARGV[1]

  local subjectID = redis.call('GET', sessionIdx)
  local removed = 0

  if subjectID then
    local subjectIdxKey = subjIdxPref .. subjectID
    local sid = string.sub(dataKey, string.len('data:session:') + 1) -- extract <sid>
    if sid and #sid > 0 then
      redis.call('SREM', subjectIdxKey, sid)
    end
  end

  removed = removed + redis.call('DEL', dataKey)
  removed = removed + redis.call('DEL', metaKey)
  removed = removed + redis.call('DEL', sessionIdx)

  return removed