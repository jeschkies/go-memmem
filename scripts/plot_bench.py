#!/usr/bin/env python3
"""Plot Go benchmark results (as produced by `go test -bench`).

Parses lines like:
  BenchmarkIndexNeedleLen/needleLen=2/haystackSize=262144/impl=avx2-32  137887  8711 ns/op  30095.13 MB/s

Repeated runs (-count=N) are aggregated by median, with min/max shown as a band.

Usage:
  python3 scripts/plot_bench.py results.log -o bench.png
  python3 scripts/plot_bench.py results.log --metric ns -o bench_ns.png
"""
import argparse
import re
from collections import defaultdict
from statistics import median

import matplotlib.pyplot as plt

LINE_RE = re.compile(
    r"^Benchmark\S*?/needleLen=(?P<needle>\d+)/haystackSize=(?P<hay>\d+)/impl=(?P<impl>\S+?)(?:-\d+)?"
    r"\s+\d+\s+(?P<ns>[\d.]+) ns/op(?:\s+(?P<mbs>[\d.]+) MB/s)?"
)


def parse(path):
    # data[needle][impl][hay] -> list of (ns, mbs)
    data = defaultdict(lambda: defaultdict(lambda: defaultdict(list)))
    with open(path) as f:
        for line in f:
            m = LINE_RE.match(line)
            if not m:
                continue
            mbs = float(m["mbs"]) if m["mbs"] else None
            data[int(m["needle"])][m["impl"]][int(m["hay"])].append((float(m["ns"]), mbs))
    return data


def human(n):
    for unit in ("B", "KiB", "MiB", "GiB"):
        if n < 1024 or unit == "GiB":
            return f"{n:g} {unit}"
        n /= 1024


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("logfile")
    ap.add_argument("-o", "--output", default="bench.png")
    ap.add_argument("--metric", choices=["mbs", "ns"], default="mbs", help="throughput (MB/s) or time (ns/op)")
    ap.add_argument("--cols", type=int, default=4)
    args = ap.parse_args()

    data = parse(args.logfile)
    if not data:
        raise SystemExit(f"no benchmark lines parsed from {args.logfile}")

    idx = 1 if args.metric == "mbs" else 0
    ylabel = "Throughput (MB/s)" if args.metric == "mbs" else "Time (ns/op)"

    needles = sorted(data)
    cols = min(args.cols, len(needles))
    rows = -(-len(needles) // cols)
    fig, axes = plt.subplots(rows, cols, figsize=(4.5 * cols, 3.6 * rows), squeeze=False, sharex=True)

    impls = sorted({i for n in data.values() for i in n})
    colors = {impl: f"C{k}" for k, impl in enumerate(impls)}

    for ax, needle in zip(axes.flat, needles):
        for impl in impls:
            series = data[needle].get(impl)
            if not series:
                continue
            xs = sorted(series)
            vals = [[s[idx] for s in series[x] if s[idx] is not None] for x in xs]
            med = [median(v) for v in vals]
            ax.plot(xs, med, marker="o", ms=4, label=impl, color=colors[impl])
            ax.fill_between(xs, [min(v) for v in vals], [max(v) for v in vals], alpha=0.2, color=colors[impl])
        ax.set_title(f"needle length = {needle}")
        ax.set_xscale("log", base=2)
        if args.metric == "ns":
            ax.set_yscale("log")
        ax.grid(True, alpha=0.3)
        ax.set_xlabel("Haystack size")
        ax.set_ylabel(ylabel)
        ticks = sorted({x for s in data[needle].values() for x in s})
        ax.set_xticks(ticks)
        ax.set_xticklabels([human(t) for t in ticks], rotation=45, ha="right", fontsize=8)
        ax.minorticks_off()

    for ax in list(axes.flat)[len(needles):]:
        ax.set_visible(False)

    handles, labels = axes.flat[0].get_legend_handles_labels()
    fig.suptitle("memmem benchmark (median, band = min–max)", y=0.995, fontsize=13)
    fig.legend(handles, labels, loc="upper center", bbox_to_anchor=(0.5, 0.955), ncol=len(labels), frameon=False)
    fig.tight_layout(rect=(0, 0, 1, 0.91))
    fig.savefig(args.output, dpi=150)
    print(f"wrote {args.output}")


if __name__ == "__main__":
    main()
