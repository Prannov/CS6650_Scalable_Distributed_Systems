#!/usr/bin/env python3
import argparse
import csv
import time
from datetime import datetime, timezone

import requests
import numpy as np
import matplotlib.pyplot as plt


def load_test(url: str, duration_seconds: int = 30, timeout_seconds: int = 10, sleep_ms: int = 0):
    """
    Sequential load test: sends back-to-back GET requests for duration_seconds.
    Records per-request latency, status code, and errors.
    """
    rows = []
    end_time = time.time() + duration_seconds
    i = 0

    print(f"Starting load test: {duration_seconds}s -> {url}")
    print(f"timeout={timeout_seconds}s, sleep_between_requests={sleep_ms}ms\n")

    session = requests.Session()

    while time.time() < end_time:
        i += 1
        ts = datetime.now(timezone.utc).isoformat()
        start = time.perf_counter()

        status = None
        err = ""
        try:
            r = session.get(url, timeout=timeout_seconds)
            status = r.status_code
        except requests.RequestException as e:
            err = str(e)

        latency_ms = (time.perf_counter() - start) * 1000.0

        rows.append({
            "request_num": i,
            "timestamp_utc": ts,
            "latency_ms": latency_ms,
            "status_code": status if status is not None else "",
            "error": err
        })

        if status == 200 and not err:
            print(f"#{i:4d}  {latency_ms:8.2f} ms  status=200")
        elif err:
            print(f"#{i:4d}  {latency_ms:8.2f} ms  ERROR: {err}")
        else:
            print(f"#{i:4d}  {latency_ms:8.2f} ms  status={status}")

        if sleep_ms > 0:
            time.sleep(sleep_ms / 1000.0)

    return rows


def write_csv(rows, path: str):
    fieldnames = ["request_num", "timestamp_utc", "latency_ms", "status_code", "error"]
    with open(path, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=fieldnames)
        w.writeheader()
        w.writerows(rows)


def summarize(rows, slow_threshold_ms: float = 500.0):
    latencies = [r["latency_ms"] for r in rows if r["error"] == "" and str(r["status_code"]) != ""]
    errors = [r for r in rows if r["error"] != ""]
    non_200 = [r for r in rows if r["error"] == "" and r["status_code"] not in ("", 200, "200")]

    print("\n=== Summary ===")
    print(f"Total requests: {len(rows)}")
    print(f"Successful responses: {len(latencies)}")
    print(f"Non-200 responses: {len(non_200)}")
    print(f"Errors/exceptions: {len(errors)}")

    if not latencies:
        print("No successful responses collected — check your URL, security group (8080), and server status.")
        return None

    arr = np.array(latencies, dtype=float)
    p50 = np.percentile(arr, 50)
    p95 = np.percentile(arr, 95)
    p99 = np.percentile(arr, 99)
    slow_pct = float(np.mean(arr > slow_threshold_ms) * 100.0)

    print(f"Mean:   {np.mean(arr):.2f} ms")
    print(f"Median: {p50:.2f} ms")
    print(f"P95:    {p95:.2f} ms")
    print(f"P99:    {p99:.2f} ms")
    print(f"Max:    {np.max(arr):.2f} ms")
    print(f"Slow(>{slow_threshold_ms:.0f}ms): {slow_pct:.2f}%")

    return arr


def plot(arr, out_png: str = None):
    # No subplots? Your template used subplots; that’s fine. This uses 2 separate figures for clarity.
    plt.figure(figsize=(12, 6))
    plt.hist(arr, bins=50, alpha=0.7)
    plt.xlabel("Response Time (ms)")
    plt.ylabel("Count")
    plt.title("Histogram of Response Times")
    if out_png:
        plt.savefig(out_png.replace(".png", "_hist.png"), dpi=150, bbox_inches="tight")
    else:
        plt.show()

    plt.figure(figsize=(12, 6))
    plt.scatter(range(len(arr)), arr, alpha=0.6)
    plt.xlabel("Request Number")
    plt.ylabel("Response Time (ms)")
    plt.title("Response Times Over Time")
    if out_png:
        plt.savefig(out_png.replace(".png", "_scatter.png"), dpi=150, bbox_inches="tight")
    else:
        plt.show()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--url", required=True, help="Example: http://<EC2_PUBLIC_IP>:8080/albums")
    ap.add_argument("--duration", type=int, default=30)
    ap.add_argument("--timeout", type=int, default=10)
    ap.add_argument("--sleep-ms", type=int, default=0, help="Optional delay between requests (ms)")
    ap.add_argument("--csv", default="load_test_results.csv")
    ap.add_argument("--slow-ms", type=float, default=500.0)
    ap.add_argument("--save-plots", action="store_true", help="Save plots as PNGs instead of showing")
    args = ap.parse_args()

    rows = load_test(args.url, args.duration, args.timeout, args.sleep_ms)

    write_csv(rows, args.csv)
    print(f"\nSaved CSV: {args.csv}")

    arr = summarize(rows, args.slow_ms)
    if arr is None:
        return

    if args.save_plots:
        plot(arr, out_png="plots.png")
        print("Saved plots: plots_hist.png, plots_scatter.png")
    else:
        plot(arr)


if __name__ == "__main__":
    main()
