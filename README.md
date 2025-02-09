# rtkv

![Build](https://github.com/johnknl/rtkv/actions/workflows/ci.yaml/badge.svg)
![Coverage](https://codecov.io/gh/johnknl/rtkv/branch/main/graph/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/johnknl/rtkv)](https://goreportcard.com/report/github.com/johnknl/rtkv)
![Go Version](https://img.shields.io/github/go-mod/go-version/johnknl/rtkv)

Small wrapper around [go-redis](https://github.com/redis/go-redis) to
get and set data in Redis by ID with a separate index for time
ordered retrieval. Suitable for large data sets.

## Fetching Ranges

There are 2 methods to fetch entities by time range:

### FetchPage()

Locally retrieves the keys of entities to fetch, then yields the
results using MGET. No read consistency when fetching pages.
In very high concurrency contexts where timestamps are updated
often, or entities often deleted, there's a non-zero chance of
getting less items than the requested page size, or a page containing
a more recent version of an entity; one that reflects a state
outside of the requested time window. This is an intentional
trade-off, as fetching pages accross different contexts will not
be consistent anyway. If you need consistent pages, use
`FetchPageConsistent()`.

The byte slices yielded by the iterator returned by this method
are not safe for reuse.

### FetchPageConsistent()

Uses a Lua script to ensure that selecting a time range and
retrieving data is done in an atomic operation. Don't reuse
the byte slices yielded. It is not possible to get a consistent
scan of a range that yields more than a little over 5k results.

## Benchmarks

These benchmarks show the difference between the 2 methods of
fetching pages. It uses the `Paginate` function to fetch a total
of 100k records in 20 pages of 5k. This was done on a single
Redis instance. Consistent fetching delegates more work (and
memory usage) to Redis, which may affect your scaling
considerations. To save you some math: fetching a single page
of 5k records takes about 2.3ms and 6.8ms respectively.

```
goos: linux
goarch: amd64
pkg: github.com/johnknl/rtkv
cpu: AMD Ryzen 9 5950X 16-Core Processor
BenchmarkRedisTKV_FetchPage
BenchmarkRedisTKV_FetchPage/Default
BenchmarkRedisTKV_FetchPage/Default-32               195          57347413 ns/op        69601861 B/op     400833 allocs/op
BenchmarkRedisTKV_FetchPage/Default-32               214          59313912 ns/op        69601491 B/op     400832 allocs/op
BenchmarkRedisTKV_FetchPage/Default-32               205          60582640 ns/op        69601354 B/op     400831 allocs/op
BenchmarkRedisTKV_FetchPage/Consistent
BenchmarkRedisTKV_FetchPage/Consistent-32             73         169228474 ns/op        59909809 B/op     200513 allocs/op
BenchmarkRedisTKV_FetchPage/Consistent-32             73         169711133 ns/op        59909780 B/op     200514 allocs/op
BenchmarkRedisTKV_FetchPage/Consistent-32             74         169084674 ns/op        59909592 B/op     200513 allocs/op
PASS
ok      github.com/johnknl/rtkv 103.021s
```

## Documentation / Usage

Documentation and usage examples are available on [pkg.go.dev](https://pkg.go.dev/github.com/johnknl/rtkv).
