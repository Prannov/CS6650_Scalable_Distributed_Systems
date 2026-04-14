#!/bin/bash
# smoke-test.sh — run after `docker compose up`
# Usage: ./scripts/smoke-test.sh [base_url]

BASE=${1:-http://localhost:8080}
ALBUM_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')

echo "=== Health ==="
curl -sf "$BASE/health" | jq .

echo ""
echo "=== Create Album ==="
curl -sf -X PUT "$BASE/albums/$ALBUM_ID" \
  -H "Content-Type: application/json" \
  -d "{\"album_id\":\"$ALBUM_ID\",\"title\":\"Test\",\"description\":\"desc\",\"owner\":\"test@neu.edu\"}" | jq .

echo ""
echo "=== Get Album ==="
curl -sf "$BASE/albums/$ALBUM_ID" | jq .

echo ""
echo "=== List Albums ==="
curl -sf "$BASE/albums" | jq 'length'

echo ""
echo "=== Upload Photo ==="
PHOTO_RESPONSE=$(curl -sf -X POST "$BASE/albums/$ALBUM_ID/photos" \
  -F "photo=@/etc/hostname;type=image/jpeg")
echo $PHOTO_RESPONSE | jq .
PHOTO_ID=$(echo $PHOTO_RESPONSE | jq -r .photo_id)

echo ""
echo "=== Poll Status (up to 30s) ==="
for i in $(seq 1 30); do
  STATUS=$(curl -sf "$BASE/albums/$ALBUM_ID/photos/$PHOTO_ID" | jq -r .status)
  echo "  attempt $i: $STATUS"
  if [ "$STATUS" = "completed" ]; then
    echo "  SUCCESS"
    break
  fi
  sleep 1
done

echo ""
echo "=== Delete Photo ==="
curl -sf -X DELETE "$BASE/albums/$ALBUM_ID/photos/$PHOTO_ID" -o /dev/null -w "HTTP %{http_code}\n"

echo ""
echo "=== Verify 404 after delete ==="
curl -sf "$BASE/albums/$ALBUM_ID/photos/$PHOTO_ID" | jq . || echo "404 as expected"

echo ""
echo "=== DONE ==="
