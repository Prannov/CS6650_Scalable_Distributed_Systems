#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get the directory where the script is located and navigate there
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

echo -e "${GREEN}Starting deployment...${NC}"
echo -e "${YELLOW}Working from: $(pwd)${NC}"

# Check if AWS credentials are configured
if ! aws sts get-caller-identity &> /dev/null; then
    echo -e "${RED}Error: AWS credentials not configured!${NC}"
    echo -e "${YELLOW}Please run 'aws configure' or set AWS environment variables${NC}"
    exit 1
fi

# Get AWS account ID and region
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
AWS_REGION=${AWS_REGION:-us-east-1}
PROJECT_NAME="product-api"

echo -e "${YELLOW}AWS Account ID: ${AWS_ACCOUNT_ID}${NC}"
echo -e "${YELLOW}AWS Region: ${AWS_REGION}${NC}"

# Check if Docker is running
if ! docker info &> /dev/null; then
    echo -e "${RED}Error: Docker is not running!${NC}"
    echo -e "${YELLOW}Please start Docker Desktop${NC}"
    exit 1
fi

# Step 1: Initialize and apply Terraform
echo -e "${GREEN}Step 1: Deploying infrastructure with Terraform...${NC}"

# Change to terraform directory
cd terraform

# Initialize Terraform if needed
if [ ! -d ".terraform" ]; then
    terraform init
fi

# Apply terraform
terraform apply -auto-approve

# Get ECR repository URL
ECR_REPO_URL=$(terraform output -raw ecr_repository_url)
echo -e "${YELLOW}ECR Repository: ${ECR_REPO_URL}${NC}"

# Go back to root directory
cd ..

# Step 2: Build Go binary for Linux AMD64
echo -e "${GREEN}Step 2: Building Go binary for Linux AMD64...${NC}"
cd src

# Remove old binary
rm -f server

# Cross-compile for Linux AMD64
echo -e "${YELLOW}Cross-compiling Go binary...${NC}"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server main.go

# Verify binary
if [ ! -f "server" ]; then
    echo -e "${RED}Error: Failed to build binary${NC}"
    exit 1
fi

echo -e "${GREEN}Binary built successfully${NC}"
file server || echo "Binary created"

# Step 3: Build and push Docker image
echo -e "${GREEN}Step 3: Building and pushing Docker image...${NC}"

# Login to ECR
echo -e "${YELLOW}Logging into ECR...${NC}"
aws ecr get-login-password --region ${AWS_REGION} | \
  docker login --username AWS --password-stdin ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com

# Build Docker image
echo -e "${YELLOW}Building Docker image...${NC}"
docker build --platform linux/amd64 -t ${PROJECT_NAME}:latest .

# Tag and push image
echo -e "${YELLOW}Pushing image to ECR...${NC}"
docker tag ${PROJECT_NAME}:latest ${ECR_REPO_URL}:latest
docker push ${ECR_REPO_URL}:latest

cd ..

# Step 4: Force new deployment
echo -e "${GREEN}Step 4: Forcing ECS service update...${NC}"
aws ecs update-service \
  --cluster ${PROJECT_NAME}-cluster \
  --service ${PROJECT_NAME}-service \
  --force-new-deployment \
  --region ${AWS_REGION} > /dev/null

echo -e "${GREEN}Deployment complete!${NC}"
echo -e "${YELLOW}Waiting for service to stabilize (this may take 2-3 minutes)...${NC}"

# Wait for service to stabilize (with timeout)
aws ecs wait services-stable \
  --cluster ${PROJECT_NAME}-cluster \
  --services ${PROJECT_NAME}-service \
  --region ${AWS_REGION} 2>/dev/null || echo -e "${YELLOW}Service is deploying (health checks may take a few minutes)${NC}"

# Get ALB URL
cd terraform
ALB_URL=$(terraform output -raw alb_url)
cd ..

echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}Deployment Complete!${NC}"
echo -e "${GREEN}================================${NC}"
echo -e "${YELLOW}API URL: ${ALB_URL}${NC}"
echo -e "${YELLOW}Health Check: ${ALB_URL}/v1/health${NC}"
echo -e ""
echo -e "${YELLOW}Wait 30 seconds for health checks, then test with:${NC}"
echo -e ""
echo -e "  # Health check"
echo -e "  curl ${ALB_URL}/v1/health"
echo -e ""
echo -e "  # Add a product"
echo -e "  curl -X POST ${ALB_URL}/v1/products/1/details \\"
echo -e "    -H 'Content-Type: application/json' \\"
echo -e "    -d '{\"sku\":\"ABC-123\",\"manufacturer\":\"Acme\",\"category_id\":10,\"weight\":500,\"some_other_id\":99}'"
echo -e ""
echo -e "  # Get a product"
echo -e "  curl ${ALB_URL}/v1/products/1"
echo -e ""
echo -e "${GREEN}View logs with:${NC}"
echo -e "  aws logs tail /ecs/product-api --follow --region ${AWS_REGION}"