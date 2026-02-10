#!/bin/bash

SPLITTER_IP="100.24.49.219"
MAPPER1_IP="44.220.51.124"
MAPPER2_IP="54.84.13.108"
MAPPER3_IP="44.198.178.197"
REDUCER_IP="98.91.223.41"
BUCKET="mapreduce-757840225451"

echo "=== Testing Services ==="
echo "Splitter: $(curl -s http://$SPLITTER_IP:8080/)"
echo "Mapper1: $(curl -s http://$MAPPER1_IP:8080/)"
echo "Mapper2: $(curl -s http://$MAPPER2_IP:8080/)"
echo "Mapper3: $(curl -s http://$MAPPER3_IP:8080/)"
echo "Reducer: $(curl -s http://$REDUCER_IP:8080/)"

echo -e "\n=== Step 1: Splitting ==="
curl -s "http://$SPLITTER_IP:8080/split?url=s3://$BUCKET/input.txt&chunks=3" | jq '.'

echo -e "\n=== Step 2: Mapping ==="
echo "Mapping chunk 0..."
curl -s "http://$MAPPER1_IP:8080/map?url=s3://$BUCKET/chunks/chunk_0.txt" | jq '.'

echo "Mapping chunk 1..."
curl -s "http://$MAPPER2_IP:8080/map?url=s3://$BUCKET/chunks/chunk_1.txt" | jq '.'

echo "Mapping chunk 2..."
curl -s "http://$MAPPER3_IP:8080/map?url=s3://$BUCKET/chunks/chunk_2.txt" | jq '.'

echo -e "\n=== Step 3: Reducing ==="
curl -s -X POST http://$REDUCER_IP:8080/reduce \
  -H "Content-Type: application/json" \
  -d "{
    \"urls\": [
      \"s3://$BUCKET/mapped/chunk_0.json\",
      \"s3://$BUCKET/mapped/chunk_1.json\",
      \"s3://$BUCKET/mapped/chunk_2.json\"
    ]
  }" | jq '.'

echo -e "\n=== Final Results ==="
aws s3 cp s3://$BUCKET/final/word_counts.json - | jq '.'