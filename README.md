# Memmem
SIMD accelerated search routines for Go

This is a rough port of Rust's [memmem](https://github.com/BurntSushi/memchr/tree/master/src/memmem).
It is roughly twice as fast as `bytes.Index`. See [benchmarks](#benchmarks) for details.

## Build Instructions

The code genereation requires [avo](https://github.com/mmcloughlin/avo):

Generate code with
```
$ go generate ./ ...
```

## Benchmarks

```
› benchstat stdlib.log simd.log
goos: linux
goarch: amd64
pkg: github.com/jeschkies/go-memmem/pkg/search
cpu: AMD Ryzen 7 3700X 8-Core Processor             
              │ stdlib.log  │              simd.log               │
              │   sec/op    │   sec/op     vs base                │
IndexSmall-16   497.3µ ± 1%   162.8µ ± 1%  -67.26% (p=0.000 n=10)
IndexBig-16     283.2n ± 1%   187.0n ± 3%  -34.00% (p=0.000 n=10)
geomean         11.87µ        5.517µ       -53.52%

              │  stdlib.log  │                simd.log                │
              │     B/s      │      B/s       vs base                 │
IndexSmall-16   7.925Gi ± 1%   24.207Gi ± 1%  +205.45% (p=0.000 n=10)
IndexBig-16     7.861Gi ± 1%   11.913Gi ± 3%   +51.54% (p=0.000 n=10)
geomean         7.893Gi         16.98Gi       +115.14%
```
