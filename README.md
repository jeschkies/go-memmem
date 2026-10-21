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
| arm64 | `simd/archsimd` (NEON), 16 bytes per vector |
| anything else | `bytes.Index` |

NEON has no movemask, so the arm64 version follows the
[AArch64 variant](http://0x80.pl/notesen/2016-11-28-simd-strfind.html#aarch64-64-bit-code)
of the article: the compare result is read as two 64-bit halves and the
matching bytes are walked in ascending order. It has not been benchmarked yet;
the numbers below are amd64 only.

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
IndexSmall-32   294.6µ ± 1%   132.4µ ± 0%  -55.05% (p=0.002 n=6)
IndexBig-32     5.302m ± 1%   1.109m ± 0%  -79.09% (p=0.002 n=6)
geomean         1.250m        383.2µ       -69.34%

              │     std      │                 simd                  │
              │     B/s      │      B/s       vs base                │
IndexSmall-32   13.38Gi ± 1%    29.76Gi ± 0%  +122.45% (p=0.002 n=6)
IndexBig-32     3.666Gi ± 1%   17.531Gi ± 0%  +378.25% (p=0.002 n=6)
geomean         7.003Gi         22.84Gi       +226.17%
```

Here `std` is `bytes.Index` and `simd` is `Index`. A negative `sec/op` change
means this library is faster. Reproduce with:

```
$ GOEXPERIMENT=simd go test ./pkg/search -run xxx -bench 'IndexSmall|IndexBig' -count 6
```
