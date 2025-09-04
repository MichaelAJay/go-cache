# Task: Redis Lua Script Execution Pattern in Go

## Objective
Implement and use Lua scripts in Redis efficiently within a Go application, ensuring scripts are not re-sent on every request and avoiding overhead from Redis cache evictions.  
Additionally, preload scripts at application startup so the first live request is never slowed by a `NOSCRIPT` fallback.

---

## Background
- Running a Lua script with `EVAL` each time sends the **entire script body** from the client to Redis, where Redis must parse and compile it.
- This introduces **bandwidth** and **CPU** overhead if done frequently.
- Redis supports an optimization pattern using **`SCRIPT LOAD` + `EVALSHA`**:
  - `SCRIPT LOAD` stores the script in Redis and returns its SHA1 hash.
  - `EVALSHA` executes the cached script by reference, without re-sending the body.
- Problem: Redis may respond with `NOSCRIPT` if the cache is flushed or Redis is restarted.
- Solution: Preload scripts on startup so Redis always knows them before handling real traffic.

---

## Pattern in `go-redis`
The `go-redis` client provides a wrapper (`redis.NewScript`) that **automatically manages script loading, caching, and fallback**:
- First call attempts `EVALSHA`.
- On `NOSCRIPT`, it retries with `EVAL`.
- SHA is then cached and used for subsequent calls.

---

## Implementation Example

```go
package main

import (
    "context"
    "fmt"
    "github.com/redis/go-redis/v9"
)

var ctx = context.Background()

// Define the Lua script
var myScript = redis.NewScript(`
    local lockKey   = KEYS[1]
    local lockValue = ARGV[1]
    local ttl       = tonumber(ARGV[2])

    local result = redis.call("SET", lockKey, lockValue, "NX", "PX", ttl)
    if result then
        return 1
    else
        return 0
    end
`)

func main() {
    rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

    // Preload the script on startup
    if _, err := myScript.Load(ctx, rdb).Result(); err != nil {
        panic(err)
    }

    // Run the script safely
    res, err := myScript.Run(ctx, rdb, []string{"mylock"}, "12345", 5000).Result()
    if err != nil {
        panic(err)
    }
    fmt.Printf("Script result: %v\n", res)
}


## Preloading Pattern

scripts := []*redis.Script{myScript, otherScript, anotherScript}
for _, s := range scripts {
    if _, err := s.Load(ctx, rdb).Result(); err != nil {
        panic(fmt.Sprintf("failed to preload script: %v", err))
    }
}


