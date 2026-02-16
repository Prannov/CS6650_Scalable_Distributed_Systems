# Product API Service - CS6650 Assignment

A scalable RESTful API for managing products, deployed on AWS ECS with Terraform infrastructure as code.

## Architecture

- **Application**: Go HTTP server with in-memory storage (HashMap)
- **Container**: Docker with Alpine Linux (minimal size)
- **Infrastructure**: AWS ECS Fargate with Application Load Balancer
- **IaC**: Terraform for automated infrastructure deployment
- **Load Testing**: Locust for performance testing

## Project Structure
```
CS6650_hw5/
├── src/
│   ├── main.go           # Go API implementation
│   ├── Dockerfile        # Container definition
│   └── go.mod           # Go module file
├── terraform/
│   ├── main.tf          # Main Terraform configuration
│   ├── provider.tf      # AWS provider configuration
│   ├── variables.tf     # Terraform variables
│   ├── output.tf        # Output values
│   └── modules/
│       ├── ecr/         # ECR repository module
│       ├── ecs/         # ECS cluster and service module
│       └── networking/  # VPC, subnets, security groups
├── tests/
│   └── locustfile.py    # Load testing scenarios
├── deploy.sh            # Automated deployment script
└── README.md
```

## Prerequisites

- AWS Account (AWS Academy Learner Lab)
- AWS CLI configured (`aws configure`)
- Terraform >= 1.0
- Docker Desktop
- Go >= 1.21
- Python 3.x with pip (for Locust)

## Deployment Instructions

### 1. Clone and Setup
```bash
git clone <your-repo-url>
cd CS6650_hw5
```

### 2. Configure AWS Credentials
```bash
aws configure
# Enter your AWS credentials when prompted
```

### 3. Deploy Infrastructure and Application
```bash
# Make deploy script executable
chmod +x deploy.sh

# Run deployment
./deploy.sh
```

The script will:
1. Initialize and apply Terraform infrastructure
2. Cross-compile Go binary for Linux AMD64
3. Build Docker image
4. Push image to Amazon ECR
5. Deploy to ECS Fargate
6. Output the API URL

### 4. Verify Deployment

Wait 30-60 seconds for health checks, then test:
```bash
# Replace with your ALB URL from deploy.sh output
export API_URL=http://your-alb-url.amazonaws.com

# Health check
curl $API_URL/v1/health

# Should return: OK
```

## API Documentation

### Base URL
```
http://<your-alb-dns-name>
```

### Endpoints

#### 1. Health Check
```bash
GET /v1/health
```

**Response: 200 OK**
```
OK
```

#### 2. Add/Update Product Details
```bash
POST /v1/products/{productId}/details
Content-Type: application/json

{
  "sku": "ABC-123",
  "manufacturer": "Acme Corp",
  "category_id": 10,
  "weight": 500,
  "some_other_id": 99
}
```

**Response: 204 No Content** (Success)

**Response: 400 Bad Request** (Invalid input)
```json
{
  "error": "INVALID_INPUT",
  "message": "SKU is required",
  "details": ""
}
```

**Response: 404 Not Found** (Product not found - though for POST this shouldn't happen)

#### 3. Get Product by ID
```bash
GET /v1/products/{productId}
```

**Response: 200 OK**
```json
{
  "product_id": 1,
  "sku": "ABC-123",
  "manufacturer": "Acme Corp",
  "category_id": 10,
  "weight": 500,
  "some_other_id": 99
}
```

**Response: 404 Not Found**
```json
{
  "error": "NOT_FOUND",
  "message": "Product not found",
  "details": "Product with ID 999 does not exist"
}
```

**Response: 400 Bad Request** (Invalid product ID)
```json
{
  "error": "INVALID_INPUT",
  "message": "Product ID must be a positive integer"
}
```

## Testing Examples

### Using curl
```bash
export API_URL=http://your-alb-url.amazonaws.com

# Add a product (returns 204)
curl -X POST $API_URL/v1/products/1/details \
  -H "Content-Type: application/json" \
  -d '{
    "sku": "LAPTOP-001",
    "manufacturer": "TechCorp",
    "category_id": 5,
    "weight": 2000,
    "some_other_id": 42
  }'

# Get the product (returns 200 with JSON)
curl $API_URL/v1/products/1 | jq

# Get non-existent product (returns 404)
curl $API_URL/v1/products/999

# Invalid product ID (returns 400)
curl $API_URL/v1/products/abc
```

### Response Code Examples

| Endpoint | Method | Scenario | Code |
|----------|--------|----------|------|
| `/v1/health` | GET | Health check | 200 |
| `/v1/products/1/details` | POST | Valid product | 204 |
| `/v1/products/1/details` | POST | Missing required field | 400 |
| `/v1/products/1` | GET | Product exists | 200 |
| `/v1/products/999` | GET | Product not found | 404 |
| `/v1/products/abc` | GET | Invalid ID format | 400 |

## Load Testing

### Setup Locust
```bash
pip install locust
```

### Run Load Tests
```bash
cd tests
locust -f locustfile.py --host=http://your-alb-url.amazonaws.com
```

Open browser to `http://localhost:8089`

### Test Configurations

**Test 1: Light Load**
- Users: 10
- Spawn rate: 1/sec
- Duration: 2 minutes

**Test 2: Medium Load**
- Users: 50
- Spawn rate: 5/sec
- Duration: 3 minutes

**Test 3: Heavy Load**
- Users: 100
- Spawn rate: 10/sec
- Duration: 5 minutes

### Load Test Results

![Locust testing - 1](/CS6650_hw5/pictures/locust_test_1.png)
![Locust testing - 1](/CS6650_hw5/pictures/locust_test_2.png)
![Locust testing - 1](/CS6650_hw5/pictures/locust_test_3.png)
![Status Codes](/CS6650_hw5/pictures/status_codes.png)
![Curl testing - 1](/CS6650_hw5/pictures/curl_testing.png)
![Curl testing - 2](/CS6650_hw5/pictures/curl_testing_2.png)

**Observations:**
- GET requests (read operations) had ~33ms average response time with median of 32-34ms
- POST requests (write operations) had ~38ms average response time with median of 32-37ms
- System handled 36.8 requests per second at peak with 50 concurrent users
- Response times remained relatively stable: 95th percentile stayed between 42-74ms across different load levels
- Read operations (GET) were slightly faster than write operations (POST) as expected with in-memory HashMap
- Health check endpoint was most consistent with minimal variance

**HttpUser vs FastHttpUser:**
- FastHttpUser showed similar performance to regular HttpUser
- At 50 users: FastHttpUser achieved 20.6 RPS for GET vs 7.7 RPS for regular HttpUser on reads
- The difference was noticeable because FastHttpUser reduces client-side overhead, allowing more requests when the server can handle them
- However, the server-side response times remained consistent (32-37ms median) regardless of client type
- This indicates the **server-side bottleneck was not CPU or memory**, but rather the single ECS task limitation (1 container with 0.25 vCPU, 0.5GB RAM)

**Key Insights:**
1. **Scalability**: With only 1 ECS task, the system handled ~37 RPS effectively
2. **Consistency**: Response time variance was low (23-132ms range shows good stability)
3. **Failures**: 14% failure rate at 50 users suggests the single task is reaching capacity
4. **Optimization Opportunity**: Increasing ECS desired count to 2-3 tasks would likely reduce failures and increase throughput significantly
5. **Data Structure Validation**: HashMap choice was correct - consistent O(1) lookup times shown by stable median response times

**Recommended Improvements:**
- Increase ECS desired count from 1 to 3 for better load distribution
- Add CloudWatch alarms for CPU >70% to trigger auto-scaling
- Implement connection pooling if adding a database
- Consider adding Redis cache for frequently accessed products

## Design Decisions

### Data Structure Choice
Used **HashMap (Go map)** for in-memory storage because:
- O(1) average time complexity for GET/PUT operations
- Read-heavy workload benefits from fast lookups
- Simple implementation for assignment scope
- Thread-safe with sync.RWMutex for concurrent access

**Trade-offs:**
- ✅ Fast read/write operations
- ✅ Simple implementation
- ❌ Data lost on container restart
- ❌ Limited to single-instance memory
- ❌ No persistence

### Architecture for Full E-commerce System

For the complete API (Products, Shopping Cart, Warehouse, Payments), I would design:

**Microservices Architecture:**
- **Product Service**: Read-heavy, use Redis caching + RDS
- **Cart Service**: Session-based, use Redis with TTL
- **Warehouse Service**: Write-heavy, use RDS with optimistic locking
- **Payment Service**: External integration, use circuit breaker pattern

**Key Components:**
1. **API Gateway**: Kong or AWS API Gateway for routing and auth
2. **Message Queue**: AWS SQS/SNS or Kafka for async processing
3. **Database**: 
   - Products: Aurora with read replicas
   - Cart: ElastiCache Redis
   - Warehouse: Aurora with ACID transactions
4. **Load Balancing**: ALB with auto-scaling ECS tasks
5. **Monitoring**: CloudWatch + X-Ray for distributed tracing

**Checkout Flow:**
```
1. Validate cart → 2. Reserve inventory (sync) → 
3. Process payment (sync) → 4. Queue shipping (async) → 
5. Update inventory → 6. Notify customer
```

## Terraform: Declarative vs Imperative

**Declarative (Terraform):**
```hcl
resource "aws_instance" "web" {
  ami           = "ami-12345"
  instance_type = "t2.micro"
}
```
You describe **what** you want (desired state). Terraform figures out **how** to achieve it.

**Imperative (Shell scripts):**
```bash
aws ec2 run-instances --image-id ami-12345 --instance-type t2.micro
```
You specify **how** to do it (step-by-step commands).

**Benefits of Declarative:**
- **Idempotent**: Running same config multiple times produces same result
- **State management**: Terraform tracks what exists
- **Dependency handling**: Automatic resource ordering
- **Plan preview**: See changes before applying
- **Drift detection**: Compare actual vs desired state

## Monitoring and Logs

### View ECS Logs
```bash
aws logs tail /ecs/product-api --follow --region us-east-1
```

### Check Service Status
```bash
aws ecs describe-services \
  --cluster product-api-cluster \
  --services product-api-service \
  --region us-east-1
```

### View CloudWatch Dashboard
Navigate to AWS Console → CloudWatch → Container Insights

## Cleanup

To avoid AWS charges:
```bash
cd terraform
terraform destroy -auto-approve
```

This removes all infrastructure: ECS cluster, ALB, ECR repository, VPC, etc.

## Lessons Learned

1. **Cross-compilation**: Building Go binaries for Linux AMD64 on ARM Mac required explicit GOOS/GOARCH flags
2. **IAM Permissions**: AWS Learner Lab requires using pre-created LabRole instead of creating custom IAM roles
3. **ECS Health Checks**: ALB health checks can take 60-90 seconds to stabilize
4. **Fargate Resources**: CPU and memory must be valid combinations (256 CPU = 512/1024/2048 MB)
5. **Load Testing**: Read operations dominated (70%+) realistic workloads, validating HashMap choice

## Contributors

- Prannov Jamadagni - Northeastern University

## License

MIT
