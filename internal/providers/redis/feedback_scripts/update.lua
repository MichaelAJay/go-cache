  local lockKey   = KEYS[1]
  local dataKey   = KEYS[2]
  local metaKey   = KEYS[3]
  local lockValue = ARGV[1]
  local ttl       = tonumber(ARGV[2])
  local newVal    = ARGV[3]
  local lockTout  = tonumber(ARGV[4])

  local ok = redis.call('SET', lockKey, lockValue, 'EX', lockTout, 'NX')
  if not ok then
    return {false, '1'} -- signal retry
  end

  local oldVal = redis.call('GET', dataKey)
  local existed = oldVal and '1' or '0'

  if ttl and ttl > 0 then
    redis.call('SETEX', dataKey, ttl, newVal)
  else
    redis.call('SET', dataKey, newVal)
  end

  local now = redis.call('TIME')
  local ts  = now[1]
  local acc = redis.call('HGET', metaKey, 'access_count') or '0'
  acc = tostring((tonumber(acc) or 0) + 1)

  redis.call('HSET', metaKey,
    'last_accessed', ts,
    'access_count', acc,
    'ttl', tostring(ttl or 0),
    'size', tostring(string.len(newVal))
  )
  if ttl and ttl > 0 then
    redis.call('EXPIRE', metaKey, ttl)
  end

  if redis.call('GET', lockKey) == lockValue then
    redis.call('DEL', lockKey)
  end

  return {oldVal, existed, newVal}