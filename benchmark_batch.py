import time
import requests
import json
import sys

# 1. Load your local STAC item
file_path = "sample_stac/test_item.json"
try:
    with open(file_path, "r") as f:
        single_item = json.load(f)
except FileNotFoundError:
    print(f"❌ Could not find {file_path}. Please check the path.")
    sys.exit(1)

url = "http://localhost:8080/validate"

# 2. Warm up the cache
print("🔥 Sending 1 item to warm up the cache (Cold Start)...")
requests.post(url, json=single_item)
print("Cache warmed up!\n")

# 3. Build a massive ItemCollection
BATCH_SIZE = 10000
print(f"📦 Building an ItemCollection with {BATCH_SIZE:,} items...")

item_collection = {
    "type": "ItemCollection",
    "features": [single_item] * BATCH_SIZE  # Duplicate the item 10,000 times
}

# Check the payload size (just to see how much data we are throwing at Go)
payload_size_mb = len(json.dumps(item_collection).encode('utf-8')) / (1024 * 1024)
print(f"Payload size: {payload_size_mb:.2f} MB")

# 4. Send the massive batch
print("\n⚡ Firing massive batch at the Go server...")
start_time = time.time()

response = requests.post(url, json=item_collection)

total_time = time.time() - start_time

# 5. Print the results
if response.status_code == 200:
    data = response.json()
    print("\n✅ Batch Processing Complete!")
    print(f"Total Items Processed: {data.get('total_processed'):,}")
    print(f"Valid Items: {data.get('valid_count'):,}")
    print(f"Invalid Items: {data.get('invalid_count'):,}")
    print("-" * 30)
    print(f"Total Time Taken: {total_time:.4f} seconds")
    print(f"Average Time per Item: {(total_time / BATCH_SIZE) * 1000:.4f} ms")
    print(f"Throughput: {int(BATCH_SIZE / total_time):,} items / second")
else:
    print(f"❌ Server Error ({response.status_code}):\n{response.text}")