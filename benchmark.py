import time
import requests
import json

# Load the exact same STAC item you just tested
with open("sample_stac/test_item.json", "r") as f:
    stac_item = json.load(f)

url = "http://localhost:8080/validate"

print("🔥 Sending First Request (Cold Start)...")
start = time.time()
requests.post(url, json=stac_item)
print(f"First request took: {(time.time() - start) * 1000:.2f} ms\n")

print("⚡ Sending 1,000 Requests (Warm Cache)...")
start_bulk = time.time()
for _ in range(1000):
    requests.post(url, json=stac_item)
total_time = time.time() - start_bulk

print(f"Total time for 1,000 items: {total_time:.2f} seconds")
print(f"Average time per STAC item: {(total_time / 1000) * 1000:.2f} ms")