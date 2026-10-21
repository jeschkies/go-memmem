# Memmem
SIMD accelerated search routines for Go

This is a rough port of Rust's [memmem](https://github.com/BurntSushi/memchr/tree/master/src/memmem).
It is roughly two to five times as fast as `bytes.Index` on the benchmark
data below, depending on the match. See [benchmarks](#benchmarks) for details.

## Algorithm

The search is the "generic SIMD" algorithm from Wojciech Muła's
[SIMD-friendly algorithms for substring searching](http://0x80.pl/notesen/2016-11-28-simd-strfind.html).
The first and last byte of the needle are broadcast into two vectors. Each
step compares them against two loads of the haystack that are `len(needle)-1`
bytes apart and combines the results into a bit mask of candidate positions.
Only at those positions are the remaining needle bytes compared.

## Platform support

| Platform | Implementation |
|---|---|
| amd64 with AVX2 | [`simd/archsimd`](https://pkg.go.dev/simd/archsimd), 32 bytes per vector |
| anything else | `bytes.Index` |

arm64 is not implemented yet and falls back to `bytes.Index`. A NEON port is
planned, following the
[AArch64 variant](http://0x80.pl/notesen/2016-11-28-simd-strfind.html#aarch64-64-bit-code)
of the same article.

## Build Instructions

The SIMD code uses Go's [`simd/archsimd`](https://pkg.go.dev/simd/archsimd)
package. It needs Go 1.27 or newer and is only compiled in with the
experiment enabled:

```
$ GOEXPERIMENT=simd go test ./...
```

Without `GOEXPERIMENT=simd`, or on a CPU without AVX2, `Index` falls back to
`bytes.Index`. There is no generated assembly.

The benchmarks depend on `data.zip` which must be checked out with [Git Large
File Storage](https://git-lfs.com/).

## Benchmarks

`IndexSmall` and `IndexBig` search a needle in a log file from `data.zip`; the
benchmark is in `pkg/search/bytes_test.go`. Read these numbers with caution:
this library is very fast when the needle is deep in a large haystack, and the
result depends on the needle and the data.

Measured on 2026-10-21 with `benchstat` (6 runs each, `bytes.Index` against this
library built with `GOEXPERIMENT=simd`):

| | |
|---|---|
| CPU | AMD Ryzen 9 9950X, 16 cores / 32 threads |
| Cache | 16 MiB L2, 64 MiB L3 |
| OS | Linux 6.19.10 (Fedora 44), amd64 |
| Go | go1.27.1 |

```
goos: linux
goarch: amd64
pkg: github.com/jeschkies/go-memmem/pkg/search
cpu: AMD Ryzen 9 9950X 16-Core Processor
              │     std     │                simd                │
              │   sec/op    │   sec/op     vs base               │
IndexSmall-32   295.8µ ± 2%   133.7µ ± 1%  -54.81% (p=0.002 n=6)
IndexBig-32     5.308m ± 1%   1.071m ± 0%  -79.82% (p=0.002 n=6)
geomean         1.253m        378.4µ       -69.80%

              │     std      │                 simd                  │
              │     B/s      │      B/s       vs base                │
IndexSmall-32   13.33Gi ± 2%    29.49Gi ± 1%  +121.29% (p=0.002 n=6)
IndexBig-32     3.661Gi ± 1%   18.141Gi ± 0%  +395.49% (p=0.002 n=6)
geomean         6.985Gi         23.13Gi       +231.13%
```

Here `std` is `bytes.Index` and `simd` is `Index`. A negative `sec/op` change
means this library is faster. Reproduce with:

```
$ GOEXPERIMENT=simd go test ./pkg/search -run xxx -bench 'IndexSmall|IndexBig' -count 6
```
