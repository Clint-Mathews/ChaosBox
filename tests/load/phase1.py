#!/usr/bin/env python3

import argparse
import json
import math
import signal
import sys
import threading
import time
import uuid
from concurrent.futures import ThreadPoolExecutor
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Optional
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


THRESHOLDS_MS = {
    "list_products": 500,
    "create_order": 1000,
    "list_orders": 1000,
}


@dataclass
class RequestMetric:
    operation: str
    status: int
    duration_ms: float
    passed: bool
    request_id: str


class Results:
    def __init__(self, verbose: bool) -> None:
        self.verbose = verbose
        self.lock = threading.Lock()
        self.requests: list[RequestMetric] = []
        self.checks_total = 0
        self.checks_passed = 0
        self.journeys_total = 0
        self.journeys_passed = 0

    def record_request(self, metric: RequestMetric, user_id: int, method: str, path: str) -> None:
        with self.lock:
            self.requests.append(metric)
            if self.verbose or not metric.passed:
                print(
                    f"[user {user_id:03d}] {method} {path} -> "
                    f"{metric.status or 'ERROR'} ({metric.duration_ms:.1f} ms) "
                    f"request_id={metric.request_id}",
                    flush=True,
                )

    def check(self, passed: bool, user_id: int, message: str) -> bool:
        with self.lock:
            self.checks_total += 1
            if passed:
                self.checks_passed += 1
            else:
                print(f"[user {user_id:03d}] check failed: {message}", file=sys.stderr, flush=True)
        return passed

    def record_journey(self, passed: bool) -> None:
        with self.lock:
            self.journeys_total += 1
            if passed:
                self.journeys_passed += 1


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run the ChaosBox Phase 1 user journey.")
    parser.add_argument("--profile", choices=("smoke", "load"), default="load")
    parser.add_argument("--base-url", default="http://localhost:8081")
    parser.add_argument("--users", type=positive_int, default=100)
    parser.add_argument("--ramp-seconds", type=non_negative_float, default=100.0)
    parser.add_argument("--hold-seconds", type=non_negative_float, default=60.0)
    parser.add_argument("--think-seconds", type=non_negative_float, default=0.2)
    parser.add_argument("--timeout-seconds", type=positive_float, default=5.0)
    parser.add_argument("--output-dir", default="artifacts/load-tests")
    parser.add_argument("--grafana-url", default="http://localhost:3000")
    parser.add_argument("--verbose", action="store_true")
    return parser.parse_args()


def positive_int(value: str) -> int:
    parsed = int(value)
    if parsed <= 0:
        raise argparse.ArgumentTypeError("must be greater than zero")
    return parsed


def positive_float(value: str) -> float:
    parsed = float(value)
    if parsed <= 0:
        raise argparse.ArgumentTypeError("must be greater than zero")
    return parsed


def non_negative_float(value: str) -> float:
    parsed = float(value)
    if parsed < 0:
        raise argparse.ArgumentTypeError("must be zero or greater")
    return parsed


def request_json(
    results: Results,
    user_id: int,
    base_url: str,
    method: str,
    path: str,
    operation: str,
    expected_status: int,
    timeout: float,
    payload: Optional[dict[str, Any]] = None,
) -> tuple[int, Any]:
    data = json.dumps(payload).encode("utf-8") if payload is not None else None
    request_id = uuid.uuid4().hex
    headers = {"X-Request-ID": request_id}
    if payload is not None:
        headers["Content-Type"] = "application/json"
    request = Request(f"{base_url}{path}", data=data, headers=headers, method=method)
    started = time.perf_counter()
    status = 0
    body = b""

    try:
        with urlopen(request, timeout=timeout) as response:
            status = response.status
            body = response.read()
            request_id = response.headers.get("X-Request-ID", request_id)
    except HTTPError as error:
        status = error.code
        body = error.read()
        request_id = error.headers.get("X-Request-ID", request_id)
    except (URLError, TimeoutError, OSError) as error:
        if results.verbose:
            print(f"[user {user_id:03d}] request error: {error}", file=sys.stderr, flush=True)

    duration_ms = (time.perf_counter() - started) * 1000
    passed = status == expected_status
    results.record_request(
        RequestMetric(
            operation=operation,
            status=status,
            duration_ms=duration_ms,
            passed=passed,
            request_id=request_id,
        ),
        user_id,
        method,
        path,
    )

    if not body:
        return status, None
    try:
        return status, json.loads(body)
    except json.JSONDecodeError:
        return status, None


def run_journey(
    results: Results,
    stop_event: threading.Event,
    user_id: int,
    iteration: int,
    base_url: str,
    think_seconds: float,
    timeout: float,
) -> bool:
    products_status, products = request_json(
        results,
        user_id,
        base_url,
        "GET",
        "/products",
        "list_products",
        200,
        timeout,
    )
    products_ok = results.check(products_status == 200, user_id, "products status is 200")
    products_ok = results.check(
        isinstance(products, list) and len(products) > 0,
        user_id,
        "products response is a non-empty array",
    ) and products_ok
    if not products_ok:
        return False

    product = products[(user_id + iteration) % len(products)]
    if stop_event.wait(think_seconds):
        return False

    order_status, order = request_json(
        results,
        user_id,
        base_url,
        "POST",
        "/orders",
        "create_order",
        201,
        timeout,
        {"items": [{"product_id": product["id"], "quantity": 1}]},
    )
    order_number = order.get("order_number") if isinstance(order, dict) else None
    order_ok = results.check(order_status == 201, user_id, "create order status is 201")
    order_ok = results.check(
        isinstance(order_number, str) and len(order_number) > 0,
        user_id,
        "create order returns an order number",
    ) and order_ok
    if not order_ok:
        return False

    if stop_event.wait(think_seconds):
        return False

    orders_status, orders = request_json(
        results,
        user_id,
        base_url,
        "GET",
        "/orders",
        "list_orders",
        200,
        timeout,
    )
    orders_ok = results.check(orders_status == 200, user_id, "orders status is 200")
    orders_ok = results.check(isinstance(orders, list), user_id, "orders response is an array") and orders_ok
    order_found = isinstance(orders, list) and any(
        item.get("order_number") == order_number for item in orders if isinstance(item, dict)
    )
    orders_ok = results.check(order_found, user_id, "created order appears in orders") and orders_ok
    return orders_ok


def run_user(
    results: Results,
    stop_event: threading.Event,
    user_id: int,
    start_delay: float,
    deadline: float,
    run_once: bool,
    base_url: str,
    think_seconds: float,
    timeout: float,
) -> None:
    if stop_event.wait(start_delay):
        return

    iteration = 0
    while not stop_event.is_set():
        passed = run_journey(
            results,
            stop_event,
            user_id,
            iteration,
            base_url,
            think_seconds,
            timeout,
        )
        results.record_journey(passed)
        iteration += 1
        if run_once or time.monotonic() >= deadline:
            return


def percentile(values: list[float], fraction: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    index = max(0, math.ceil(len(ordered) * fraction) - 1)
    return ordered[index]


def build_summary(results: Results, args: argparse.Namespace, elapsed_seconds: float) -> dict[str, Any]:
    requests_total = len(results.requests)
    requests_failed = sum(1 for metric in results.requests if not metric.passed)
    check_rate = results.checks_passed / results.checks_total if results.checks_total else 0.0
    failure_rate = requests_failed / requests_total if requests_total else 1.0
    journey_rate = results.journeys_passed / results.journeys_total if results.journeys_total else 0.0
    status_counts: dict[str, int] = {}
    for metric in results.requests:
        status = str(metric.status) if metric.status else "transport_error"
        status_counts[status] = status_counts.get(status, 0) + 1

    operations = {}
    latency_thresholds_passed = True
    for operation, threshold_ms in THRESHOLDS_MS.items():
        durations = [metric.duration_ms for metric in results.requests if metric.operation == operation]
        p95_ms = percentile(durations, 0.95)
        passed = bool(durations) and p95_ms < threshold_ms
        latency_thresholds_passed = latency_thresholds_passed and passed
        operations[operation] = {
            "requests": len(durations),
            "average_ms": round(sum(durations) / len(durations), 2) if durations else 0.0,
            "p95_ms": round(p95_ms, 2),
            "max_ms": round(max(durations), 2) if durations else 0.0,
            "threshold_ms": threshold_ms,
            "passed": passed,
        }

    thresholds = {
        "checks_at_least_99_percent": check_rate >= 0.99,
        "request_failures_below_1_percent": failure_rate < 0.01,
        "latencies_within_limits": latency_thresholds_passed,
    }

    return {
        "profile": args.profile,
        "base_url": args.base_url,
        "users": 1 if args.profile == "smoke" else args.users,
        "elapsed_seconds": round(elapsed_seconds, 2),
        "requests": {
            "total": requests_total,
            "failed": requests_failed,
            "failure_rate": round(failure_rate, 4),
            "status_counts": status_counts,
            "representative_failures": [
                asdict(metric) for metric in results.requests if not metric.passed
            ][:20],
        },
        "checks": {
            "total": results.checks_total,
            "passed": results.checks_passed,
            "rate": round(check_rate, 4),
        },
        "journeys": {
            "total": results.journeys_total,
            "passed": results.journeys_passed,
            "rate": round(journey_rate, 4),
        },
        "operations": operations,
        "thresholds": thresholds,
        "passed": all(thresholds.values()),
        "generated_at": datetime.now(timezone.utc).isoformat(),
    }


def print_summary(summary: dict[str, Any], output_path: Path, dashboard_url: str) -> None:
    print("\nPhase 1 load-test summary")
    print(f"Profile: {summary['profile']}")
    print(f"Users: {summary['users']}")
    print(f"Elapsed: {summary['elapsed_seconds']:.2f}s")
    print(
        f"Requests: {summary['requests']['total']} total, "
        f"{summary['requests']['failed']} failed "
        f"({summary['requests']['failure_rate'] * 100:.2f}%)"
    )
    print(
        f"Checks: {summary['checks']['passed']}/{summary['checks']['total']} "
        f"({summary['checks']['rate'] * 100:.2f}%)"
    )
    print(
        f"Journeys: {summary['journeys']['passed']}/{summary['journeys']['total']} "
        f"({summary['journeys']['rate'] * 100:.2f}%)"
    )
    for operation, metrics in summary["operations"].items():
        print(
            f"{operation}: p95={metrics['p95_ms']:.2f}ms, "
            f"max={metrics['max_ms']:.2f}ms, threshold={metrics['threshold_ms']}ms"
        )
    print(f"Result: {'PASS' if summary['passed'] else 'FAIL'}")
    print(f"Summary: {output_path}")
    print(f"Grafana: {dashboard_url}")


def main() -> int:
    args = parse_args()
    args.base_url = args.base_url.rstrip("/")
    if args.profile == "smoke":
        args.verbose = True

    results = Results(verbose=args.verbose)
    stop_event = threading.Event()

    def stop(_signum: int, _frame: Any) -> None:
        stop_event.set()

    signal.signal(signal.SIGINT, stop)
    signal.signal(signal.SIGTERM, stop)

    users = 1 if args.profile == "smoke" else args.users
    run_once = args.profile == "smoke"
    started = time.monotonic()
    started_at_ms = int(time.time() * 1000)
    deadline = started + (0 if run_once else args.ramp_seconds + args.hold_seconds)

    print(
        f"Running {args.profile} profile against {args.base_url} "
        f"with {users} user{'s' if users != 1 else ''}",
        flush=True,
    )

    with ThreadPoolExecutor(max_workers=users) as executor:
        futures = []
        for user_index in range(users):
            start_delay = 0.0
            if users > 1:
                start_delay = args.ramp_seconds * user_index / (users - 1)
            futures.append(
                executor.submit(
                    run_user,
                    results,
                    stop_event,
                    user_index + 1,
                    start_delay,
                    deadline,
                    run_once,
                    args.base_url,
                    args.think_seconds,
                    args.timeout_seconds,
                )
            )
        for future in futures:
            future.result()

    elapsed_seconds = time.monotonic() - started
    finished_at_ms = int(time.time() * 1000)
    summary = build_summary(results, args, elapsed_seconds)
    run_id = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ") + f"-{args.profile}"
    output_dir = Path(args.output_dir) / run_id
    output_dir.mkdir(parents=True, exist_ok=True)
    output_path = output_dir / "summary.json"
    dashboard_url = (
        f"{args.grafana_url.rstrip('/')}/d/chaosbox-load-test/chaosbox-load-test"
        f"?from={started_at_ms - 5000}&to={finished_at_ms + 5000}"
    )
    summary["grafana_url"] = dashboard_url
    output_path.write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    print_summary(summary, output_path, dashboard_url)
    return 0 if summary["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
