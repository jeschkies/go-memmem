# Memmem
SIMD accelerated search routines for Go

This is a rough port of Rust's [memmem](https://github.com/BurntSushi/memchr/tree/master/src/memmem).
It is roughly four times as fast as `bytes.Index` depending on the match.
See [benchmarks](#benchmarks) for details.

## Build Instructions

The code genereation requires [avo](https://github.com/mmcloughlin/avo):

Generate code with
```
$ go generate ./ ...
```

The benchmarks depend on `data.zip` which must be checked out with [Git Large
File Storage](https://git-lfs.com/).

## Benchmarks

As always read these benchmarks with caution. This library is very fast when the
needle is deep in a large haystack.

```
› benchstat stdlib.log simd.log                                                                         
goos: linux
goarch: amd64
pkg: github.com/jeschkies/go-memmem/pkg/search
cpu: AMD Ryzen 7 3700X 8-Core Processor             
              │ stdlib.log  │              simd.log              │
              │   sec/op    │   sec/op     vs base               │
IndexSmall-16   485.5µ ± 2%   128.4µ ± 3%  -73.55% (p=0.002 n=6)
IndexBig-16     9.224m ± 1%   1.243m ± 1%  -86.52% (p=0.002 n=6)
geomean         2.116m        399.5µ       -81.12%

              │  stdlib.log  │               simd.log                │
              │     B/s      │      B/s       vs base                │
IndexSmall-16   8.118Gi ± 2%   30.698Gi ± 3%  +278.13% (p=0.002 n=6)
IndexBig-16     2.107Gi ± 1%   15.631Gi ± 1%  +641.87% (p=0.002 n=6)
geomean         4.136Gi         21.91Gi       +429.64%

```
