#!/bin/bash
# Usage: ./deploy.sh <mysql|dynamodb>
set -e

BACKEND=${1:-mysql}
REGION="us-east-1"
ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
ECR_URL="$ACCOUNT.dkr.ecr.$REGION.amazonaws.com/hw8-cart-service"

echo "=== Step 1: Terraform (backend=$BACKEND) ==="
terraform init -input=false
terraform apply -auto-approve \
  -var="db_backend=$BACKEND" \
  -var="db_password=Password123!"

ALB_URL=$(terraform output -raw alb_url)
echo "ALB: $ALB_URL"

echo "=== Step 2: Build + Push Docker image ==="
# cross-compile from ARM Mac → linux/amd64
docker buildx build --platform linux/amd64 -t hw8-cart:latest --load .

aws ecr get-login-password --region $REGION \
  | docker login --username AWS --password-stdin "$ACCOUNT.dkr.ecr.$REGION.amazonaws.com"

docker tag hw8-cart:latest "$ECR_URL:latest"
docker push "$ECR_URL:latest"

echo "=== Step 3: Force ECS redeploy ==="
aws ecs update-service \
  --cluster hw8-cluster \
  --service hw8-cart-service \
  --force-new-deployment \
  --region $REGION > /dev/null

echo "Waiting for service stable (~2 min)..."
aws ecs wait services-stable \
  --cluster hw8-cluster \
  --services hw8-cart-service \
  --region $REGION

echo ""
echo "=== Ready ==="
echo "ALB URL: $ALB_URL"
echo "Run test: python3 test.py $ALB_URL $BACKEND"