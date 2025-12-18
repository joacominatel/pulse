import argparse
import asyncio
import random
import statistics
import sys
import time
from dataclasses import dataclass, field
from typing import Optional

try:
    import aiohttp
except ImportError:
    print("error: aiohttp is required. install with: pip install aiohttp")
    sys.exit(1)


# default configuration
DEFAULT_BASE_URL = "http://localhost:8080"
DEFAULT_CONCURRENCY = 100
DEFAULT_DURATION_SECONDS = 30

# event types with weights for realistic distribution
EVENT_TYPES = [
    ("view", 40),
    ("join", 5),
    ("post", 10),
    ("comment", 20),
    ("reaction", 25),
    ("share", 5),
]


@dataclass
class LatencyStats:
    """tracks latency metrics for a test scenario"""
    latencies: list = field(default_factory=list)
    successes: int = 0
    failures: int = 0
    errors: list = field(default_factory=list)
    
    def record_success(self, latency_ms: float):
        self.latencies.append(latency_ms)
        self.successes += 1
    
    def record_failure(self, error: str):
        self.failures += 1
        if len(self.errors) < 10:  # keep first 10 errors for debugging
            self.errors.append(error)
    
    def percentile(self, p: float) -> float:
        if not self.latencies:
            return 0.0
        sorted_latencies = sorted(self.latencies)
        idx = int(len(sorted_latencies) * p / 100)
        idx = min(idx, len(sorted_latencies) - 1)
        return sorted_latencies[idx]
    
    def summary(self) -> dict:
        if not self.latencies:
            return {
                "total_requests": self.successes + self.failures,
                "successes": self.successes,
                "failures": self.failures,
                "error_rate": 100.0 if self.failures > 0 else 0.0,
            }
        
        return {
            "total_requests": self.successes + self.failures,
            "successes": self.successes,
            "failures": self.failures,
            "error_rate": round(self.failures / (self.successes + self.failures) * 100, 2),
            "min_ms": round(min(self.latencies), 2),
            "max_ms": round(max(self.latencies), 2),
            "mean_ms": round(statistics.mean(self.latencies), 2),
            "median_ms": round(statistics.median(self.latencies), 2),
            "p50_ms": round(self.percentile(50), 2),
            "p90_ms": round(self.percentile(90), 2),
            "p95_ms": round(self.percentile(95), 2),
            "p99_ms": round(self.percentile(99), 2),
            "requests_per_second": round(self.successes / (max(self.latencies) / 1000) if self.latencies else 0, 2),
        }


def weighted_random_event() -> str:
    """select event type based on realistic distribution"""
    total = sum(w for _, w in EVENT_TYPES)
    r = random.randint(1, total)
    cumulative = 0
    for event_type, weight in EVENT_TYPES:
        cumulative += weight
        if r <= cumulative:
            return event_type
    return "view"


async def fetch_communities(
    session: aiohttp.ClientSession,
    base_url: str,
    token: str,
) -> list:
    """fetch available communities for load testing"""
    url = f"{base_url}/api/v1/communities"
    headers = {"Authorization": f"Bearer {token}"}
    params = {"limit": 50}
    
    try:
        async with session.get(url, headers=headers, params=params) as resp:
            if resp.status == 200:
                data = await resp.json()
                return data.get("communities", [])
            return []
    except Exception:
        return []


async def ingestion_worker(
    worker_id: int,
    session: aiohttp.ClientSession,
    base_url: str,
    token: str,
    community_ids: list,
    stats: LatencyStats,
    stop_event: asyncio.Event,
):
    """worker that continuously sends POST /events requests"""
    url = f"{base_url}/api/v1/events"
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }
    
    while not stop_event.is_set():
        community_id = random.choice(community_ids)
        event_type = weighted_random_event()
        
        payload = {
            "community_id": community_id,
            "event_type": event_type,
            "metadata": {
                "worker_id": worker_id,
                "source": "load_test",
            },
        }
        
        start = time.perf_counter()
        try:
            async with session.post(url, json=payload, headers=headers, timeout=aiohttp.ClientTimeout(total=10)) as resp:
                latency_ms = (time.perf_counter() - start) * 1000
                
                if resp.status in (200, 201, 202):
                    stats.record_success(latency_ms)
                else:
                    body = await resp.text()
                    stats.record_failure(f"status={resp.status}: {body[:100]}")
        except asyncio.TimeoutError:
            stats.record_failure("timeout")
        except Exception as e:
            stats.record_failure(str(e)[:100])
        
        # small jitter to avoid thundering herd
        await asyncio.sleep(random.uniform(0.001, 0.01))


async def discovery_worker(
    worker_id: int,
    session: aiohttp.ClientSession,
    base_url: str,
    token: str,
    stats: LatencyStats,
    stop_event: asyncio.Event,
):
    """worker that continuously sends GET /communities requests"""
    url = f"{base_url}/api/v1/communities"
    headers = {"Authorization": f"Bearer {token}"}
    
    while not stop_event.is_set():
        # vary the limit to simulate different client behaviors
        limit = random.choice([10, 20, 50])
        offset = random.choice([0, 0, 0, 10, 20])  # most requests are first page
        
        params = {"limit": limit, "offset": offset}
        
        start = time.perf_counter()
        try:
            async with session.get(url, headers=headers, params=params, timeout=aiohttp.ClientTimeout(total=10)) as resp:
                latency_ms = (time.perf_counter() - start) * 1000
                
                if resp.status == 200:
                    stats.record_success(latency_ms)
                else:
                    body = await resp.text()
                    stats.record_failure(f"status={resp.status}: {body[:100]}")
        except asyncio.TimeoutError:
            stats.record_failure("timeout")
        except Exception as e:
            stats.record_failure(str(e)[:100])
        
        # small jitter
        await asyncio.sleep(random.uniform(0.001, 0.01))


async def run_ingestion_test(
    base_url: str,
    token: str,
    concurrency: int,
    duration: int,
    community_ids: list,
) -> LatencyStats:
    """run scenario A: ingestion load test"""
    print(f"\n{'='*60}")
    print("SCENARIO A: INGESTION LOAD TEST")
    print(f"{'='*60}")
    print(f"concurrency: {concurrency} workers")
    print(f"duration: {duration} seconds")
    print(f"target communities: {len(community_ids)}")
    print(f"endpoint: POST /api/v1/events")
    print()
    
    stats = LatencyStats()
    stop_event = asyncio.Event()
    
    connector = aiohttp.TCPConnector(limit=concurrency + 50, limit_per_host=concurrency + 50)
    async with aiohttp.ClientSession(connector=connector) as session:
        # start all workers
        workers = [
            asyncio.create_task(
                ingestion_worker(i, session, base_url, token, community_ids, stats, stop_event)
            )
            for i in range(concurrency)
        ]
        
        # progress reporting
        start_time = time.time()
        while time.time() - start_time < duration:
            elapsed = int(time.time() - start_time)
            print(f"\r  progress: {elapsed}/{duration}s | requests: {stats.successes + stats.failures} | errors: {stats.failures}", end="", flush=True)
            await asyncio.sleep(1)
        
        print()
        
        # signal workers to stop
        stop_event.set()
        
        # give workers a moment to finish
        await asyncio.sleep(0.5)
        
        # cancel remaining tasks
        for worker in workers:
            worker.cancel()
        
        # wait for cancellation
        await asyncio.gather(*workers, return_exceptions=True)
    
    return stats


async def run_discovery_test(
    base_url: str,
    token: str,
    concurrency: int,
    duration: int,
) -> LatencyStats:
    """run scenario B: discovery load test"""
    print(f"\n{'='*60}")
    print("SCENARIO B: DISCOVERY LOAD TEST")
    print(f"{'='*60}")
    print(f"concurrency: {concurrency} workers")
    print(f"duration: {duration} seconds")
    print(f"endpoint: GET /api/v1/communities")
    print()
    
    stats = LatencyStats()
    stop_event = asyncio.Event()
    
    connector = aiohttp.TCPConnector(limit=concurrency + 50, limit_per_host=concurrency + 50)
    async with aiohttp.ClientSession(connector=connector) as session:
        # start all workers
        workers = [
            asyncio.create_task(
                discovery_worker(i, session, base_url, token, stats, stop_event)
            )
            for i in range(concurrency)
        ]
        
        # progress reporting
        start_time = time.time()
        while time.time() - start_time < duration:
            elapsed = int(time.time() - start_time)
            print(f"\r  progress: {elapsed}/{duration}s | requests: {stats.successes + stats.failures} | errors: {stats.failures}", end="", flush=True)
            await asyncio.sleep(1)
        
        print()
        
        # signal workers to stop
        stop_event.set()
        
        # give workers a moment to finish
        await asyncio.sleep(0.5)
        
        # cancel remaining tasks
        for worker in workers:
            worker.cancel()
        
        # wait for cancellation
        await asyncio.gather(*workers, return_exceptions=True)
    
    return stats


def print_results(scenario: str, stats: LatencyStats, duration: int):
    """print formatted test results"""
    summary = stats.summary()
    
    print(f"\n{'─'*60}")
    print(f"RESULTS: {scenario.upper()}")
    print(f"{'─'*60}")
    
    print(f"\n  Total Requests:     {summary['total_requests']:,}")
    print(f"  Successful:         {summary['successes']:,}")
    print(f"  Failed:             {summary['failures']:,}")
    print(f"  Error Rate:         {summary['error_rate']}%")
    
    if stats.latencies:
        actual_rps = summary['successes'] / duration
        print(f"\n  Throughput:         {actual_rps:,.1f} req/s")
        
        print(f"\n  Latency (ms):")
        print(f"    Min:              {summary['min_ms']}")
        print(f"    Mean:             {summary['mean_ms']}")
        print(f"    Median (p50):     {summary['p50_ms']}")
        print(f"    p90:              {summary['p90_ms']}")
        print(f"    p95:              {summary['p95_ms']}")
        print(f"    p99:              {summary['p99_ms']}")
        print(f"    Max:              {summary['max_ms']}")
    
    if stats.errors:
        print(f"\n  Sample Errors ({len(stats.errors)}):")
        for err in stats.errors[:5]:
            print(f"    - {err}")
    
    print()


async def main():
    parser = argparse.ArgumentParser(
        description="load testing for pulse api",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
examples:
  # test ingestion with 1000 concurrent users for 30 seconds
  python load_test.py -s ingestion -c 1000 -d 30 -t <jwt_token>
  
  # test discovery with 5000 concurrent users
  python load_test.py -s discovery -c 5000 -d 30 -t <jwt_token>
  
  # test both scenarios
  python load_test.py -s both -c 1000 -d 60 -t <jwt_token>
        """,
    )
    
    parser.add_argument(
        "-s", "--scenario",
        choices=["ingestion", "discovery", "both"],
        default="both",
        help="test scenario to run (default: both)",
    )
    parser.add_argument(
        "-c", "--concurrency",
        type=int,
        default=DEFAULT_CONCURRENCY,
        help=f"number of concurrent workers (default: {DEFAULT_CONCURRENCY})",
    )
    parser.add_argument(
        "-d", "--duration",
        type=int,
        default=DEFAULT_DURATION_SECONDS,
        help=f"test duration in seconds (default: {DEFAULT_DURATION_SECONDS})",
    )
    parser.add_argument(
        "-u", "--url",
        default=DEFAULT_BASE_URL,
        help=f"base url of the api (default: {DEFAULT_BASE_URL})",
    )
    parser.add_argument(
        "-t", "--token",
        required=True,
        help="jwt token for authentication (required)",
    )
    parser.add_argument(
        "--community-ids",
        nargs="+",
        help="specific community IDs to target (optional, fetches from api if not provided)",
    )
    
    args = parser.parse_args()
    
    print("\n" + "="*60)
    print("PULSE LOAD TESTING")
    print("="*60)
    print(f"target: {args.url}")
    print(f"scenario: {args.scenario}")
    print(f"concurrency: {args.concurrency}")
    print(f"duration: {args.duration}s")
    
    # fetch communities for ingestion test
    community_ids = args.community_ids
    if args.scenario in ("ingestion", "both") and not community_ids:
        print("\nfetching communities from api...")
        async with aiohttp.ClientSession() as session:
            communities = await fetch_communities(session, args.url, args.token)
            if not communities:
                print("error: no communities found. create some first.")
                sys.exit(1)
            community_ids = [c["id"] for c in communities]
            print(f"found {len(community_ids)} communities")
    
    # run tests
    if args.scenario == "ingestion":
        stats = await run_ingestion_test(
            args.url, args.token, args.concurrency, args.duration, community_ids
        )
        print_results("ingestion", stats, args.duration)
        
    elif args.scenario == "discovery":
        stats = await run_discovery_test(
            args.url, args.token, args.concurrency, args.duration
        )
        print_results("discovery", stats, args.duration)
        
    elif args.scenario == "both":
        # run ingestion first
        ingestion_stats = await run_ingestion_test(
            args.url, args.token, args.concurrency, args.duration // 2, community_ids
        )
        print_results("ingestion", ingestion_stats, args.duration // 2)
        
        # brief pause to let things settle
        print("\npausing 5s before discovery test...")
        await asyncio.sleep(5)
        
        # run discovery
        discovery_stats = await run_discovery_test(
            args.url, args.token, args.concurrency, args.duration // 2
        )
        print_results("discovery", discovery_stats, args.duration // 2)
        
        # combined summary
        print("\n" + "="*60)
        print("COMBINED SUMMARY")
        print("="*60)
        print(f"\nIngestion: p95={ingestion_stats.percentile(95):.2f}ms, p99={ingestion_stats.percentile(99):.2f}ms")
        print(f"Discovery: p95={discovery_stats.percentile(95):.2f}ms, p99={discovery_stats.percentile(99):.2f}ms")
        
        # overall assessment
        ingestion_p99 = ingestion_stats.percentile(99)
        discovery_p99 = discovery_stats.percentile(99)
        
        print("\n" + "-"*60)
        if ingestion_p99 < 100 and discovery_p99 < 50:
            print("✓ PASS: both scenarios within acceptable latency targets")
            print(f"  - ingestion p99 < 100ms: {ingestion_p99:.2f}ms")
            print(f"  - discovery p99 < 50ms: {discovery_p99:.2f}ms (redis serving)")
        else:
            print("✗ REVIEW: latency targets may not be met")
            if ingestion_p99 >= 100:
                print(f"  - ingestion p99 >= 100ms: {ingestion_p99:.2f}ms")
            if discovery_p99 >= 50:
                print(f"  - discovery p99 >= 50ms: {discovery_p99:.2f}ms")
        print("-"*60)
    
    print("\nload test complete.\n")


if __name__ == "__main__":
    asyncio.run(main())
