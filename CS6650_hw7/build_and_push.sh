#!/bin/bash
set -e

REGION=${AWS_REGION:-us-east-1}
ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
REGISTRY="$ACCOUNT.dkr.ecr.$REGION.amazonaws.com"

echo "==> Logging into ECR..."
aws ecr get-login-password --region $REGION | docker login --username AWS --password-stdin $REGISTRY

# ── Build binaries locally (cross-compiled for linux/amd64) ─────────────────

echo "==> Building order-receiver binary..."
cd order-receiver
go mod tidy
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o receiver .
cd ..

echo "==> Building order-processor binary..."
cd order-processor
go mod tidy
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o processor .
cd ..

# ── Build Lambda zip ─────────────────────────────────────────────────────────

echo "==> Building Lambda binary..."
cd order-lambda
go mod tidy
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bootstrap .
zip function.zip bootstrap
cd ..

# ── Docker build + ECR push ──────────────────────────────────────────────────

for svc in order-receiver order-processor; do
  echo "==> Creating ECR repo $svc (if not exists)..."
  aws ecr describe-repositories --repository-names $svc --region $REGION 2>/dev/null || \
    aws ecr create-repository --repository-name $svc --region $REGION

  echo "==> Building Docker image for $svc..."
  docker build -t $svc ./$svc

  echo "==> Pushing $svc to ECR..."
  docker tag $svc:latest $REGISTRY/$svc:latest
  docker push $REGISTRY/$svc:latest
done

echo ""
echo "Done! Paste these into terraform/terraform.tfvars:"
echo "ecr_repo_receiver  = \"$REGISTRY/order-receiver\""
echo "ecr_repo_processor = \"$REGISTRY/order-processor\""