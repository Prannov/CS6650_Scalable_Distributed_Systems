#!/bin/bash
set -e

# ── Config ──────────────────────────────────────────────────────
REGION="us-east-1"
REPO_NAME="product-search"
# ────────────────────────────────────────────────────────────────

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
ECR_URI="$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/$REPO_NAME"

echo "→ Creating ECR repo (if it doesn't exist)..."
aws ecr describe-repositories --repository-names $REPO_NAME --region $REGION 2>/dev/null || \
  aws ecr create-repository --repository-name $REPO_NAME --region $REGION

echo "→ Logging in to ECR..."
aws ecr get-login-password --region $REGION | \
  docker login --username AWS --password-stdin "$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com"

echo "→ Building image for linux/amd64..."
docker buildx build --platform linux/amd64 -t $REPO_NAME:latest .

echo "→ Tagging & pushing..."
docker tag $REPO_NAME:latest $ECR_URI:latest
docker push $ECR_URI:latest

echo ""
echo "✅ Done! Image URI:"
echo "   $ECR_URI:latest"
echo ""
echo "Paste this into terraform.tfvars as image_uri"