import json
import statistics

# Load benchmark data
with open("benchmark_results.json", "r") as f:
    data = json.load(f)

put_times = [req["time_ms"] for req in data["put_requests"]]
get_times = [req["time_ms"] for req in data["get_requests"]]
delete_times = [req["time_ms"] for req in data.get("delete_requests", [])]

def calculate_percentiles(times):
    if not times:
        return None
    
    sorted_times = sorted(times)
    p50 = statistics.median(sorted_times)
    p95 = sorted_times[int(len(sorted_times) * 0.95)] if len(sorted_times) > 0 else 0
    p99 = sorted_times[int(len(sorted_times) * 0.99)] if len(sorted_times) > 0 else 0
    
    return {
        "p50": round(p50, 2),
        "p95": round(p95, 2),
        "p99": round(p99, 2),
        "mean": round(statistics.mean(sorted_times), 2),
        "min": round(min(sorted_times), 2),
        "max": round(max(sorted_times), 2),
        "count": len(sorted_times)
    }

print("=== PUT Requests ===")
put_stats = calculate_percentiles(put_times)
if put_stats:
    print(f"Count:  {put_stats['count']}")
    print(f"Min:    {put_stats['min']}ms")
    print(f"Max:    {put_stats['max']}ms")
    print(f"Mean:   {put_stats['mean']}ms")
    print(f"P50:    {put_stats['p50']}ms")
    print(f"P95:    {put_stats['p95']}ms")
    print(f"P99:    {put_stats['p99']}ms")
else:
    print("No PUT requests found")

print("\n=== GET Requests ===")
get_stats = calculate_percentiles(get_times)
if get_stats:
    print(f"Count:  {get_stats['count']}")
    print(f"Min:    {get_stats['min']}ms")
    print(f"Max:    {get_stats['max']}ms")
    print(f"Mean:   {get_stats['mean']}ms")
    print(f"P50:    {get_stats['p50']}ms")
    print(f"P95:    {get_stats['p95']}ms")
    print(f"P99:    {get_stats['p99']}ms")
else:
    print("No GET requests found")

print("\n=== DELETE Requests ===")
delete_stats = calculate_percentiles(delete_times)
if delete_stats:
    print(f"Count:  {delete_stats['count']}")
    print(f"Min:    {delete_stats['min']}ms")
    print(f"Max:    {delete_stats['max']}ms")
    print(f"Mean:   {delete_stats['mean']}ms")
    print(f"P50:    {delete_stats['p50']}ms")
    print(f"P95:    {delete_stats['p95']}ms")
    print(f"P99:    {delete_stats['p99']}ms")
else:
    print("No DELETE requests found")

# Save analysis to file
analysis_data = {
    "put_stats": put_stats,
    "get_stats": get_stats,
    "delete_stats": delete_stats
}

with open("benchmark_analysis.json", "w") as f:
    json.dump(analysis_data, f, indent=2)

print("\nAnalysis saved to benchmark_analysis.json")
