#!/usr/bin/env python3
"""
Run: python3 test.py <ALB_URL> <mysql|dynamodb>
Outputs: mysql_test_results.json or dynamodb_test_results.json
"""
import sys, json, time, datetime, urllib.request, urllib.error

BASE_URL = sys.argv[1].rstrip("/")
BACKEND  = sys.argv[2]          # mysql or dynamodb
OUT_FILE = f"{BACKEND}_test_results.json"
RESULTS  = []

def req(method, path, body=None):
    url  = f"{BASE_URL}{path}"
    data = json.dumps(body).encode() if body else None
    r    = urllib.request.Request(url, data=data, method=method,
                                  headers={"Content-Type": "application/json"})
    start = time.time()
    try:
        with urllib.request.urlopen(r, timeout=10) as resp:
            ms      = (time.time() - start) * 1000
            payload = json.loads(resp.read())
            return ms, resp.status, payload, True
    except urllib.error.HTTPError as e:
        ms = (time.time() - start) * 1000
        return ms, e.code, {}, False
    except Exception as e:
        ms = (time.time() - start) * 1000
        return ms, 0, {}, False

def record(op, ms, ok, code):
    RESULTS.append({
        "operation":     op,
        "response_time": round(ms, 2),
        "success":       ok,
        "status_code":   code,
        "timestamp":     datetime.datetime.utcnow().isoformat() + "Z"
    })

def run():
    cart_ids = []

    # ── 50 create_cart ────────────────────────────────────────────────────────
    print("Creating 50 carts...")
    for i in range(50):
        ms, code, body, ok = req("POST", "/shopping-carts",
                                 {"customer_id": f"cust-{i:04d}"})
        record("create_cart", ms, ok, code)
        if ok:
            cart_ids.append(body["cart_id"])
        if (i+1) % 10 == 0:
            print(f"  {i+1}/50 done")

    # ── 50 add_items ──────────────────────────────────────────────────────────
    print("Adding items to 50 carts...")
    for i in range(50):
        cid = cart_ids[i % len(cart_ids)]
        ms, code, _, ok = req("POST", f"/shopping-carts/{cid}/items", {
            "items": [
                {"product_id": f"prod-{i%10:03d}", "name": f"Product {i%10}",
                 "quantity": 1, "price": 9.99 + i % 5}
            ]
        })
        record("add_items", ms, ok, code)
        if (i+1) % 10 == 0:
            print(f"  {i+1}/50 done")

    # ── 50 get_cart ───────────────────────────────────────────────────────────
    print("Retrieving 50 carts...")
    for i in range(50):
        cid = cart_ids[i % len(cart_ids)]
        ms, code, _, ok = req("GET", f"/shopping-carts/{cid}")
        record("get_cart", ms, ok, code)
        if (i+1) % 10 == 0:
            print(f"  {i+1}/50 done")

    # ── save ──────────────────────────────────────────────────────────────────
    with open(OUT_FILE, "w") as f:
        json.dump(RESULTS, f, indent=2)
    print(f"\nSaved {len(RESULTS)} results → {OUT_FILE}")

    # ── summary ───────────────────────────────────────────────────────────────
    for op in ["create_cart", "add_items", "get_cart"]:
        times = [r["response_time"] for r in RESULTS if r["operation"] == op and r["success"]]
        if times:
            times.sort()
            p95 = times[int(len(times)*0.95)]
            print(f"  {op}: avg={sum(times)/len(times):.1f}ms  p95={p95:.1f}ms  ok={len(times)}/50")

if __name__ == "__main__":
    if len(sys.argv) != 3 or sys.argv[2] not in ("mysql", "dynamodb"):
        print("usage: python3 test.py <ALB_URL> <mysql|dynamodb>")
        sys.exit(1)
    run()