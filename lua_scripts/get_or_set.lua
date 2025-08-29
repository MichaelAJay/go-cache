  local lockKey       = KEYS[1]
  local dataKey       = KEYS[2]
  local metaKey       = KEYS[3]
  local lockValue     = ARGV[1]
  local ttl           = tonumber(ARGV[2])
  local serializedVal = ARGV[3]
  local lockTimeout   = tonumber(ARGV[4])

  -- Fast path: already present
  local existing = redis.call('GET', dataKey)
  if existing then
    return {existing, '0'}  -- found, no write
  end

  -- Acquire lock
  local ok = redis.call('SET', lockKey, lockValue, 'EX', lockTimeout, 'NX')
  if not ok then
    -- Someone else is loading; check again
    existing = redis.call('GET', dataKey)
    if existing then
      return {existing, '0'}
    end
    return {false, '1'} -- signal retry
  end

  -- Set value
  if ttl and ttl > 0 then
    redis.call('SETEX', dataKey, ttl, serializedVal)
  else
    redis.call('SET', dataKey, serializedVal)
  end

  -- Metadata
  local now = redis.call('TIME')
  local ts  = now[1]
  redis.call('HSET', metaKey,
    'created_at', ts,
    'last_accessed', ts,
    'access_count', '1',
    'ttl', tostring(ttl or 0),
    'size', tostring(string.len(serializedVal))
  )
  if ttl and ttl > 0 then
    redis.call('EXPIRE', metaKey, ttl)
  end

  -- Safe unlock (only if we still own it)
  if redis.call('GET', lockKey) == lockValue then
    redis.call('DEL', lockKey)
  end

  return {serializedVal, '0'}