# Core Latency & Performance Benchmarks

This directory provides tools and test suites to measure hot-path latency, throughput, and memory allocations in the Torana core gateway.

---

## 1. In-Process Benchmarks

The benchmark suite (`test/bench/bench_test.go`) measures raw component overhead and reports throughput (RPS), memory allocations, and latency percentiles (p50, p99, p99.9).

### Run All Benchmarks
```bash
make bench
# Or directly via Go:
go test -run=^$ -bench=. -benchmem ./test/bench/...
```

### Run Component Microbenchmarks
```bash
# Router match overhead (radix tree lookup):
go test -run=^$ -bench=BenchmarkRouterMatch -benchmem ./test/bench/...

# Filter pipeline execution overhead (0, 1, and 5 filters):
go test -run=^$ -bench=BenchmarkPipelineChain -benchmem ./test/bench/...

# HTTP proxy hot path with mock upstream (measures p50, p99, p99.9):
go test -run=^$ -bench=BenchmarkHTTP_Proxy -benchmem ./test/bench/...

# SSE streaming chunk throughput:
go test -run=^$ -bench=BenchmarkSSE_Stream -benchmem ./test/bench/...
```

### Key Metrics Reported
- `ns/op`: Nanoseconds per operation.
- `B/op` & `allocs/op`: Heap allocation bytes and count per request.
- `p50_us`, `p99_us`, `p99.9_us`: Latency percentiles in microseconds.
- `rps`: Requests per second under concurrent load.

---

## 2. Live Gateway End-to-End Latency

To test actual network latency against a running gateway instance:

### Step 1: Start the Gateway
```bash
make build
./bin/gateway-data
```

### Step 2: Run an HTTP Load Tester

Using [`hey`](https://github.com/rakyll/hey):
```bash
# 50,000 requests, 50 concurrent workers
hey -n 50000 -c 50 \
  -m POST \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"ping"}]}' \
  http://localhost:3000/v1/chat/completions
```

**Sample Results (Apple M1, 50,000 requests, 50 concurrent workers, local run with upstream):**
| Metric | Value |
| :--- | :--- |
| **Throughput** | **23,743 req/sec** |
| **Total Duration** | 2.1059s |
| **Average Latency** | 2.1 ms |
| **p50 Latency** | 1.9 ms |
| **p75 Latency** | 2.5 ms |
| **p90 Latency** | 3.1 ms |
| **p95 Latency** | 3.6 ms |
| **p99 Latency** | 4.9 ms |
| **Fastest / Slowest** | 0.2 ms / 28.0 ms |

Using [`wrk`](https://github.com/wg/wrk):
```bash
# 4 threads, 50 connections, 30 seconds
wrk -t4 -c50 -d30s --latency \
  -s scripts/wrk_post.lua \
  http://localhost:8080/v1/chat/completions
```

---

## 3. Profiling (CPU & Memory)

To generate runtime profiles under load:

```bash
# Generate CPU and heap profiles into test/bench/out/
make profile

# Inspect interactively in browser
go tool pprof -http=:6060 test/bench/out/cpu.prof
go tool pprof -http=:6061 test/bench/out/mem.prof
```
