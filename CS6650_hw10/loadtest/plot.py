#!/usr/bin/env python3
"""
Generate report graphs from load test JSON results.
Usage: python loadtest/plot.py
Run from repo root with results/ directory present.
"""

import json
import os
import numpy as np
import matplotlib.pyplot as plt
import matplotlib.gridspec as gridspec
from collections import defaultdict

RESULTS_DIR = "results"
PLOTS_DIR   = "plots"
os.makedirs(PLOTS_DIR, exist_ok=True)

CONFIGS    = ["W5R1", "W1R5", "W3R3", "Leaderless"]
WRITE_PCTS = [1, 10, 50, 90]
COLORS     = {"W5R1": "#2196F3", "W1R5": "#4CAF50", "W3R3": "#FF9800", "Leaderless": "#E91E63"}

# ── Load all results ──────────────────────────────────────────────────────────

def load(config, writes):
    path = os.path.join(RESULTS_DIR, f"{config}_{writes}w.json")
    with open(path) as f:
        return json.load(f)

all_data = {}
for cfg in CONFIGS:
    for w in WRITE_PCTS:
        all_data[(cfg, w)] = load(cfg, w)

# ── Helper ────────────────────────────────────────────────────────────────────

def split(records):
    reads  = [r["LatencyMs"] for r in records if r["Op"] == "read"]
    writes = [r["LatencyMs"] for r in records if r["Op"] == "write"]
    stale  = sum(1 for r in records if r["Op"] == "read" and r.get("Stale"))
    return reads, writes, stale

def cdf(data):
    s = np.sort(data)
    y = np.arange(1, len(s)+1) / len(s)
    return s, y

# ── Figure 1: Read latency CDFs per config, one subplot per write% ────────────

fig, axes = plt.subplots(2, 2, figsize=(14, 10))
fig.suptitle("Read Latency CDF — by Config & Write %", fontsize=14, fontweight="bold")

for ax, w in zip(axes.flat, WRITE_PCTS):
    for cfg in CONFIGS:
        reads, _, _ = split(all_data[(cfg, w)])
        if reads:
            x, y = cdf(reads)
            ax.plot(x, y, label=cfg, color=COLORS[cfg], linewidth=1.8)
    ax.set_title(f"Writes={w}%  Reads={100-w}%")
    ax.set_xlabel("Latency (ms)")
    ax.set_ylabel("CDF")
    ax.set_xlim(left=0)
    ax.legend(fontsize=8)
    ax.grid(True, alpha=0.3)

plt.tight_layout()
plt.savefig(f"{PLOTS_DIR}/fig1_read_latency_cdf.png", dpi=150)
plt.close()
print("Saved fig1_read_latency_cdf.png")

# ── Figure 2: Write latency CDFs per config, one subplot per write% ───────────

fig, axes = plt.subplots(2, 2, figsize=(14, 10))
fig.suptitle("Write Latency CDF — by Config & Write %", fontsize=14, fontweight="bold")

for ax, w in zip(axes.flat, WRITE_PCTS):
    for cfg in CONFIGS:
        _, writes, _ = split(all_data[(cfg, w)])
        if writes:
            x, y = cdf(writes)
            ax.plot(x, y, label=cfg, color=COLORS[cfg], linewidth=1.8)
    ax.set_title(f"Writes={w}%  Reads={100-w}%")
    ax.set_xlabel("Latency (ms)")
    ax.set_ylabel("CDF")
    ax.set_xlim(left=0)
    ax.legend(fontsize=8)
    ax.grid(True, alpha=0.3)

plt.tight_layout()
plt.savefig(f"{PLOTS_DIR}/fig2_write_latency_cdf.png", dpi=150)
plt.close()
print("Saved fig2_write_latency_cdf.png")

# ── Figure 3: Stale read % per config per write ratio ─────────────────────────

fig, ax = plt.subplots(figsize=(10, 6))
fig.suptitle("Stale Read Rate by Config & Write %", fontsize=14, fontweight="bold")

x = np.arange(len(WRITE_PCTS))
width = 0.2

for i, cfg in enumerate(CONFIGS):
    stale_pcts = []
    for w in WRITE_PCTS:
        reads, _, stale = split(all_data[(cfg, w)])
        pct = (stale / len(reads) * 100) if reads else 0
        stale_pcts.append(pct)
    ax.bar(x + i*width, stale_pcts, width, label=cfg, color=COLORS[cfg])

ax.set_xticks(x + width*1.5)
ax.set_xticklabels([f"W={w}% R={100-w}%" for w in WRITE_PCTS])
ax.set_ylabel("Stale Reads (%)")
ax.set_ylim(0, 100)
ax.legend()
ax.grid(True, axis="y", alpha=0.3)

plt.tight_layout()
plt.savefig(f"{PLOTS_DIR}/fig3_stale_reads.png", dpi=150)
plt.close()
print("Saved fig3_stale_reads.png")

# ── Figure 4: Avg read & write latency heatmap ────────────────────────────────

fig, axes = plt.subplots(1, 2, figsize=(14, 5))
fig.suptitle("Average Latency Heatmap (ms)", fontsize=14, fontweight="bold")

for ax, op in zip(axes, ["read", "write"]):
    matrix = []
    for cfg in CONFIGS:
        row = []
        for w in WRITE_PCTS:
            lats = [r["LatencyMs"] for r in all_data[(cfg, w)] if r["Op"] == op]
            row.append(np.mean(lats) if lats else 0)
        matrix.append(row)

    matrix = np.array(matrix)
    im = ax.imshow(matrix, aspect="auto", cmap="YlOrRd")
    ax.set_xticks(range(len(WRITE_PCTS)))
    ax.set_xticklabels([f"W={w}%" for w in WRITE_PCTS])
    ax.set_yticks(range(len(CONFIGS)))
    ax.set_yticklabels(CONFIGS)
    ax.set_title(f"{op.capitalize()} Latency")
    plt.colorbar(im, ax=ax, label="ms")

    for i in range(len(CONFIGS)):
        for j in range(len(WRITE_PCTS)):
            ax.text(j, i, f"{matrix[i,j]:.1f}", ha="center", va="center",
                    fontsize=8, color="black")

plt.tight_layout()
plt.savefig(f"{PLOTS_DIR}/fig4_latency_heatmap.png", dpi=150)
plt.close()
print("Saved fig4_latency_heatmap.png")

# ── Figure 5: Read latency histograms showing long tail ───────────────────────

fig, axes = plt.subplots(len(CONFIGS), len(WRITE_PCTS), figsize=(18, 12))
fig.suptitle("Read Latency Distribution (Long Tail View)", fontsize=14, fontweight="bold")

for i, cfg in enumerate(CONFIGS):
    for j, w in enumerate(WRITE_PCTS):
        ax = axes[i][j]
        reads, _, _ = split(all_data[(cfg, w)])
        if reads:
            p99 = np.percentile(reads, 99)
            ax.hist(reads, bins=50, color=COLORS[cfg], alpha=0.7, edgecolor="none")
            ax.axvline(p99, color="red", linestyle="--", linewidth=1, label=f"p99={p99:.0f}ms")
            ax.legend(fontsize=6)
        if i == 0:
            ax.set_title(f"W={w}%", fontsize=9)
        if j == 0:
            ax.set_ylabel(cfg, fontsize=9)
        ax.set_xlabel("ms", fontsize=7)
        ax.tick_params(labelsize=7)

plt.tight_layout()
plt.savefig(f"{PLOTS_DIR}/fig5_read_latency_histograms.png", dpi=150)
plt.close()
print("Saved fig5_read_latency_histograms.png")

# ── Figure 6: Distribution of read-write intervals per config ─────────────────

fig, axes = plt.subplots(2, 2, figsize=(14, 10))
fig.suptitle("Distribution of Time Interval Between Write and Read of Same Key (ms)",
             fontsize=13, fontweight="bold")

for ax, cfg in zip(axes.flat, CONFIGS):
    for w in WRITE_PCTS:
        intervals = [r["IntervalMs"] for r in all_data[(cfg, w)]
                     if r["Op"] == "read" and r.get("IntervalMs", 0) > 0]
        if intervals:
            x, y = cdf(intervals)
            ax.plot(x, y, label=f"W={w}%", linewidth=1.6)
    ax.set_title(cfg)
    ax.set_xlabel("Interval (ms)")
    ax.set_ylabel("CDF")
    ax.set_xlim(left=0)
    ax.legend(fontsize=8)
    ax.grid(True, alpha=0.3)

plt.tight_layout()
plt.savefig(f"{PLOTS_DIR}/fig6_rw_interval_cdf.png", dpi=150)
plt.close()
print("Saved fig6_rw_interval_cdf.png")

# ── Print summary table ───────────────────────────────────────────────────────

print("\n=== Summary Table ===")
print(f"{'Config':<12} {'W%':>4} {'Reads':>8} {'Writes':>8} {'AvgRd(ms)':>10} {'AvgWr(ms)':>10} {'Stale%':>8}")
print("-" * 70)
for cfg in CONFIGS:
    for w in WRITE_PCTS:
        reads, writes, stale = split(all_data[(cfg, w)])
        avg_r = np.mean(reads)  if reads  else 0
        avg_w = np.mean(writes) if writes else 0
        stale_pct = (stale / len(reads) * 100) if reads else 0
        print(f"{cfg:<12} {w:>4} {len(reads):>8} {len(writes):>8} {avg_r:>10.2f} {avg_w:>10.2f} {stale_pct:>7.1f}%")