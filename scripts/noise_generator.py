import argparse
import random
import time
import sys
from typing import Optional
import requests

# default configuration
DEFAULT_BASE_URL = "http://localhost:8080"
DEFAULT_EVENTS_COUNT = 50
DEFAULT_DELAY_MS = 100

# available event types with their relative weights for random selection
# higher weight = more likely to be selected (simulates realistic distribution)
EVENT_TYPES = [
    ("view", 40),      # most common
    ("join", 5),       # rare but impactful
    ("post", 10),      # moderate
    ("comment", 20),   # common
    ("reaction", 25),  # very common
    ("share", 5),      # rare
]


def weighted_random_event() -> str:
    """select an event type based on realistic distribution"""
    total = sum(w for _, w in EVENT_TYPES)
    r = random.randint(1, total)
    cumulative = 0
    for event_type, weight in EVENT_TYPES:
        cumulative += weight
        if r <= cumulative:
            return event_type
    return "view"


def random_weight() -> Optional[float]:
    if random.random() < 0.3:
        return round(random.uniform(0.1, 5.0), 2)
    return None


def random_metadata() -> dict:
    if random.random() < 0.5:
        return {
            "source": random.choice(["web", "mobile", "api"]),
            "session_id": f"sess_{random.randint(1000, 9999)}",
        }
    return {}


def send_event(
    base_url: str,
    token: str,
    community_id: str,
    event_type: str,
    weight: Optional[float] = None,
    metadata: Optional[dict] = None,
) -> dict:
    url = f"{base_url}/api/v1/events"
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }
    payload = {
        "community_id": community_id,
        "event_type": event_type,
    }
    if weight is not None:
        payload["weight"] = weight
    if metadata:
        payload["metadata"] = metadata

    response = requests.post(url, json=payload, headers=headers, timeout=10)
    return {
        "status": response.status_code,
        "body": response.json() if response.content else {},
    }


def fetch_communities(base_url: str, token: str, limit: int = 20) -> list:
    url = f"{base_url}/api/v1/communities"
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }
    params = {"limit": limit}
    
    response = requests.get(url, headers=headers, params=params, timeout=10)
    if response.status_code != 200:
        print(f"failed to fetch communities: {response.status_code}")
        return []
    
    data = response.json()
    return data.get("communities", [])


def generate_noise(
    base_url: str,
    token: str,
    community_ids: list,
    count: int,
    delay_ms: int,
    verbose: bool = False,
):
    print(f"\ngenerating {count} events across {len(community_ids)} community(ies)...")
    print(f"   delay between events: {delay_ms}ms\n")
    
    stats = {
        "success": 0,
        "failed": 0,
        "by_type": {},
        "by_community": {},
    }
    
    for i in range(count):
        community_id = random.choice(community_ids)
        event_type = weighted_random_event()
        weight = random_weight()
        metadata = random_metadata()
        
        try:
            result = send_event(
                base_url, token, community_id, event_type, weight, metadata
            )
            
            if result["status"] == 201:
                stats["success"] += 1
                stats["by_type"][event_type] = stats["by_type"].get(event_type, 0) + 1
                stats["by_community"][community_id] = stats["by_community"].get(community_id, 0) + 1
                
                if verbose:
                    print(f"  [{i+1}/{count}] {event_type} -> {community_id[:8]}...")
            else:
                stats["failed"] += 1
                if verbose:
                    print(f"  [{i+1}/{count}] {event_type} failed: {result['body']}")
        
        except Exception as e:
            stats["failed"] += 1
            if verbose:
                print(f"  [{i+1}/{count}] error: {e}")
        
        # progress indicator
        if not verbose and (i + 1) % 10 == 0:
            print(f"   progress: {i+1}/{count} ({(i+1)/count*100:.0f}%)")
        
        # delay between events
        if delay_ms > 0 and i < count - 1:
            time.sleep(delay_ms / 1000)
    
    return stats


def print_stats(stats: dict):
    """print a summary of the noise generation"""
    print("\n" + "=" * 50)
    print("NOISE GENERATION SUMMARY")
    print("=" * 50)
    print(f"   successful: {stats['success']}")
    print(f"   failed: {stats['failed']}")
    
    if stats["by_type"]:
        print("\n   events by type:")
        for event_type, count in sorted(stats["by_type"].items(), key=lambda x: -x[1]):
            print(f"      {event_type}: {count}")
    
    if len(stats["by_community"]) > 1:
        print("\n   events by community:")
        for comm_id, count in sorted(stats["by_community"].items(), key=lambda x: -x[1]):
            print(f"      {comm_id[:8]}...: {count}")
    
    print("=" * 50 + "\n")


def main():
    parser = argparse.ArgumentParser(
        description="Generate noise events for Pulse momentum testing",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Generate 50 events for a specific community
  python noise_generator.py --community-id abc123 --token eyJ...

  # Generate 100 events randomly across all communities
  python noise_generator.py --random --count 100 --token eyJ...

  # Verbose mode with custom delay
  python noise_generator.py --community-id abc123 --token eyJ... --verbose --delay 50
        """,
    )
    
    # target selection (mutually exclusive)
    target_group = parser.add_mutually_exclusive_group(required=True)
    target_group.add_argument(
        "--community-id", "-c",
        help="Target community ID (UUID)",
    )
    target_group.add_argument(
        "--random", "-r",
        action="store_true",
        help="Distribute events randomly across all available communities",
    )
    
    # authentication
    parser.add_argument(
        "--token", "-t",
        required=True,
        help="JWT bearer token for authentication",
    )
    
    # optional parameters
    parser.add_argument(
        "--base-url", "-u",
        default=DEFAULT_BASE_URL,
        help=f"API base URL (default: {DEFAULT_BASE_URL})",
    )
    parser.add_argument(
        "--count", "-n",
        type=int,
        default=DEFAULT_EVENTS_COUNT,
        help=f"Number of events to generate (default: {DEFAULT_EVENTS_COUNT})",
    )
    parser.add_argument(
        "--delay", "-d",
        type=int,
        default=DEFAULT_DELAY_MS,
        help=f"Delay between events in ms (default: {DEFAULT_DELAY_MS})",
    )
    parser.add_argument(
        "--verbose", "-v",
        action="store_true",
        help="Print detailed output for each event",
    )
    
    args = parser.parse_args()
    
    # determine target communities
    if args.random:
        print(f"fetching communities from {args.base_url}...")
        communities = fetch_communities(args.base_url, args.token)
        
        if not communities:
            print("no communities found. create some first or check your token.")
            sys.exit(1)
        
        community_ids = [c["id"] for c in communities]
        print(f"   found {len(community_ids)} communities")
    else:
        community_ids = [args.community_id]
    
    # generate noise
    stats = generate_noise(
        base_url=args.base_url,
        token=args.token,
        community_ids=community_ids,
        count=args.count,
        delay_ms=args.delay,
        verbose=args.verbose,
    )
    
    # print summary
    print_stats(stats)
    
    # exit with error code if all failed
    if stats["success"] == 0:
        sys.exit(1)


if __name__ == "__main__":
    main()
