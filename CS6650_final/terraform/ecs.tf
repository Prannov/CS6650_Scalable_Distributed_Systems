# ── ECS Cluster ───────────────────────────────────────────────────────────────
resource "aws_ecs_cluster" "main" {
  name = "${var.app_name}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

# ── Shared env locals ─────────────────────────────────────────────────────────
locals {
  common_env = [
    { name = "DATABASE_URL", value = "postgres://album:${var.db_password}@${aws_db_instance.postgres.endpoint}/albumstore" },
    { name = "REDIS_ADDR",   value = "${aws_elasticache_cluster.redis.cache_nodes[0].address}:6379" },
    { name = "SQS_QUEUE_URL", value = aws_sqs_queue.photo_queue.url },
    { name = "S3_BUCKET",    value = aws_s3_bucket.photos.bucket },
    { name = "AWS_REGION",   value = var.aws_region },
  ]

  log_config = {
    logDriver = "awslogs"
    options = {
      "awslogs-group"         = aws_cloudwatch_log_group.app.name
      "awslogs-region"        = var.aws_region
      "awslogs-stream-prefix" = "ecs"
    }
  }
}

# ── API Task Definition ───────────────────────────────────────────────────────
resource "aws_ecs_task_definition" "api" {
  family                   = "${var.app_name}-api"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.api_cpu
  memory                   = var.api_memory
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([{
    name      = "api"
    image     = var.api_image
    essential = true
    portMappings = [{
      containerPort = 8080
      protocol      = "tcp"
    }]
    environment = concat(local.common_env, [
      { name = "PORT", value = "8080" }
    ])
    logConfiguration = local.log_config
    # Allow large multipart uploads
    ulimits = [{
      name      = "nofile"
      softLimit = 65536
      hardLimit = 65536
    }]
  }])
}

# ── API ECS Service ───────────────────────────────────────────────────────────
resource "aws_ecs_service" "api" {
  name            = "${var.app_name}-api"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = var.api_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.api.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.api.arn
    container_name   = "api"
    container_port   = 8080
  }

  depends_on = [aws_lb_listener.http]

  # Allow rolling deploys without downtime
  deployment_minimum_healthy_percent = 50
  deployment_maximum_percent         = 200
}

# ── Worker Task Definition ────────────────────────────────────────────────────
resource "aws_ecs_task_definition" "worker" {
  family                   = "${var.app_name}-worker"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.worker_cpu
  memory                   = var.worker_memory
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([{
    name      = "worker"
    image     = var.worker_image
    essential = true
    environment = local.common_env
    logConfiguration = local.log_config
  }])
}

# ── Worker ECS Service ────────────────────────────────────────────────────────
resource "aws_ecs_service" "worker" {
  name            = "${var.app_name}-worker"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.worker.arn
  desired_count   = var.worker_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.api.id]
    assign_public_ip = false
  }
}
