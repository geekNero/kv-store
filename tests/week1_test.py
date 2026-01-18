import requests
import sys
import time
import json

test_file = sys.argv[1]

tests = ""

with open(test_file, "r") as file:
    tests = file.read()


tests = tests.split("\n")

print(len(tests))

put_requests = []
get_requests = []
put_count = 0

for test in tests:
    test = test.split()
    if len(test) == 0:
        continue
    base_url = "http://localhost:8000/keystore/" + test[1]

    if test[0] == "PUT":
        data = {"value": test[2]}

        start_time = time.time()
        response = requests.put(base_url, json=data)
        elapsed_time = time.time() - start_time
        put_count += 1

        if response.status_code != 200:
            print("failed at test case:", test)
            sys.exit(1)
        
        put_requests.append({
            "num": put_count,
            "time_ms": round(elapsed_time * 1000, 2)
        })

    elif test[0] == "GET":
        start_time = time.time()
        response = requests.get(base_url)
        elapsed_time = time.time() - start_time

        if test[2] == "NOT_FOUND" and response.status_code != 404:
            print("failed at test case:", test)
            sys.exit(1)

        if test[2] != "NOT_FOUND" and test[2] != response.text.strip('"'):
            print("failed at test case:", test)
            sys.exit(1)
        
        get_requests.append({
            "put_count": put_count,
            "time_ms": round(elapsed_time * 1000, 2)
        })

print("All tests passed")

# Save benchmark data to JSON file
benchmark_data = {
    "put_requests": put_requests,
    "get_requests": get_requests
}

with open("benchmark_results.json", "w") as f:
    json.dump(benchmark_data, f, indent=2)
