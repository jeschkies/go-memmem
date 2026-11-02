#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "matplotlib==3.11.2",
#   "pandas==3.1.0",
#   "seaborn==0.13.2",
# ]
# ///
"""Plot Go benchmark results (as produced by `go test -bench`) with seaborn.

Repeated runs (-count=N) are aggregated by median, with min/max shown as a band.

Usage (dependencies are installed automatically by uv):
  uv run scripts/plot_bench.py results.log -o bench.png
  uv run scripts/plot_bench.py results.log --metric ns -o bench_ns.png
"""
import argparse
import re

import matplotlib.ticker as mticker
import pandas as pd
import seaborn as sns

LINE_RE = re.compile(
    r"^Benchmark\S*?/needleLen=(?P<needle>\d+)/haystackSize=(?P<hay>\d+)/impl=(?P<impl>\S+?)(?:-\d+)?"
    r"\s+\d+\s+(?P<ns>[\d.]+) ns/op(?:\s+(?P<mbs>[\d.]+) MB/s)?"
)


def parse(path):
    rows = []
    with open(path) as f:
        for line in f:
            if m := LINE_RE.match(line):
                rows.append((
                    int(m["needle"]), int(m["hay"]), m["impl"],
                    float(m["ns"]), float(m["mbs"]) if m["mbs"] else float("nan"),
                ))
    return pd.DataFrame(rows, columns=["needle", "hay", "impl", "ns", "mbs"])


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

    df = parse(args.logfile)
    if df.empty:
        raise SystemExit(f"no benchmark lines parsed from {args.logfile}")

    sns.set_theme(style="whitegrid")
    g = sns.relplot(
        data=df, kind="line", x="hay", y=args.metric, hue="impl", hue_order=sorted(df["impl"].unique()),
        col="needle", col_wrap=min(args.cols, df["needle"].nunique()),
        estimator="median", errorbar=("pi", 100), marker="o",  # pi=100 -> min..max
        height=3.4, aspect=1.3, facet_kws=dict(sharey=True),
    )
    g.set(xscale="log")
    if args.metric == "ns":
        g.set(yscale="log")

    ticks = sorted(df["hay"].unique())
    for ax in g.axes.flat:
        ax.set_xscale("log", base=2)
        ax.set_xticks(ticks)
        ax.set_xticklabels([human(t) for t in ticks], rotation=45, ha="right", fontsize=8)
        ax.xaxis.set_minor_locator(mticker.NullLocator())
        ax.xaxis.set_minor_formatter(mticker.NullFormatter())
    g.set_titles("needle length = {col_name}")
    g.set_axis_labels("Haystack size", "Throughput (MB/s)" if args.metric == "mbs" else "Time (ns/op)")

    sns.move_legend(g, "upper center", bbox_to_anchor=(0.5, 0.955), ncol=df["impl"].nunique(), title=None, frameon=False)
    g.figure.suptitle("memmem benchmark (median, band = min–max)", y=0.995, fontsize=13)
    g.figure.tight_layout(rect=(0, 0, 1, 0.91))
    g.figure.savefig(args.output, dpi=150)
    print(f"wrote {args.output}")


if __name__ == "__main__":
    main()
