A robust integration test system must be created for the RedisCache's methods. It must use existing container-based Redis instance functionality (see /Users/michaeljay/go-dev/go-cache/CONTAINER_TESTING.md and /Users/michaeljay/go-dev/go-cache/demo_container.go). This process needs to go in phases.
Phase 1) Simple system setup and new cache initialization. Ensure that a cache instance can be created and initialized. Ensure that Lua script warming can work.

DO NOT WRITE OR RUN TESTS. DO NOT BUILD CODE. DO NOT FIX ANY TANGENTIAL OR PARALLEL ISSUES.

Phase 2) Simple SET tests
