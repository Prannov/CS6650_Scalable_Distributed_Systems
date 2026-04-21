locals {
  db_url = "postgres://sp_user:${var.db_password}@${aws_db_instance.postgres.endpoint}/sportspulse?sslmode=require"
}

# ── ECS Cluster ───────────────────────────────────────────
resource "aws_ecs_cluster" "main" {
  name = "sportspulse-cluster"
}

# ── ALB ───────────────────────────────────────────────────
resource "aws_lb" "main" {
  name               = "sportspulse-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = [aws_subnet.public_a.id, aws_subnet.public_b.id]
  tags               = { Name = "sportspulse-alb" }
}

# ── Target Groups ─────────────────────────────────────────
resource "aws_lb_target_group" "event_svc" {
  name        = "sportspulse-event-svc-tg"
  port        = 8080
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"
  health_check {
    path                = "/health"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    interval            = 30
  }
}

resource "aws_lb_target_group" "query_svc" {
  name        = "sportspulse-query-svc-tg"
  port        = 8081
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"
  health_check {
    path                = "/health"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    interval            = 30
  }
}

# ── ALB Listeners ─────────────────────────────────────────
resource "aws_lb_listener" "event_svc" {
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.event_svc.arn
  }
}

resource "aws_lb_listener" "query_svc" {
  load_balancer_arn = aws_lb.main.arn
  port              = 8081
  protocol          = "HTTP"
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.query_svc.arn
  }
}

# ── CloudWatch Log Groups ─────────────────────────────────
resource "aws_cloudwatch_log_group" "event_svc" {
  name              = "/ecs/sportspulse-event-svc"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "stats_worker" {
  name              = "/ecs/sportspulse-stats-worker"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "query_svc" {
  name              = "/ecs/sportspulse-query-svc"
  retention_in_days = 7
}

# ── Task Definitions ──────────────────────────────────────
resource "aws_ecs_task_definition" "event_svc" {
  family                   = "sportspulse-event-svc"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = "arn:aws:iam::${var.account_id}:role/LabRole"
  task_role_arn            = "arn:aws:iam::${var.account_id}:role/LabRole"
  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }

  container_definitions = jsonencode([{
    name  = "event-svc"
    image = "${aws_ecr_repository.event_svc.repository_url}:latest"
    portMappings = [{ containerPort = 8080, hostPort = 8080 }]
    environment = [
      { name = "KAFKA_BROKER", value = "kafka:9092" },
      { name = "KAFKA_TOPIC",  value = "game-events" },
      { name = "PORT",         value = "8080" }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = "/ecs/sportspulse-event-svc"
        awslogs-region        = var.region
        awslogs-stream-prefix = "ecs"
      }
    }
  }])
}

resource "aws_ecs_task_definition" "stats_worker" {
  family                   = "sportspulse-stats-worker"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = "arn:aws:iam::${var.account_id}:role/LabRole"
  task_role_arn            = "arn:aws:iam::${var.account_id}:role/LabRole"
  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }

  container_definitions = jsonencode([{
    name  = "stats-worker"
    image = "${aws_ecr_repository.stats_worker.repository_url}:latest"
    environment = [
      { name = "KAFKA_BROKER", value = "kafka:9092" },
      { name = "KAFKA_TOPIC",  value = "game-events" },
      { name = "KAFKA_GROUP",  value = "stats-workers" },
      { name = "DB_URL",       value = local.db_url }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = "/ecs/sportspulse-stats-worker"
        awslogs-region        = var.region
        awslogs-stream-prefix = "ecs"
      }
    }
  }])
}

resource "aws_ecs_task_definition" "query_svc" {
  family                   = "sportspulse-query-svc"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = "arn:aws:iam::${var.account_id}:role/LabRole"
  task_role_arn            = "arn:aws:iam::${var.account_id}:role/LabRole"
  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }

  container_definitions = jsonencode([{
    name  = "query-svc"
    image = "${aws_ecr_repository.query_svc.repository_url}:latest"
    portMappings = [{ containerPort = 8081, hostPort = 8081 }]
    environment = [
      { name = "DB_URL",    value = local.db_url },
      { name = "REDIS_URL", value = "redis:6379" },
      { name = "PORT",      value = "8081" }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = "/ecs/sportspulse-query-svc"
        awslogs-region        = var.region
        awslogs-stream-prefix = "ecs"
      }
    }
  }])
}

# ── ECS Services ──────────────────────────────────────────
resource "aws_ecs_service" "event_svc" {
  name            = "sportspulse-event-svc"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.event_svc.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_a.id, aws_subnet.public_b.id]
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.event_svc.arn
    container_name   = "event-svc"
    container_port   = 8080
  }
}

resource "aws_ecs_service" "stats_worker" {
  name            = "sportspulse-stats-worker"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.stats_worker.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_a.id, aws_subnet.public_b.id]
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = true
  }
}

resource "aws_ecs_service" "query_svc" {
  name            = "sportspulse-query-svc"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.query_svc.arn
  desired_count   = var.query_svc_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_a.id, aws_subnet.public_b.id]
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.query_svc.arn
    container_name   = "query-svc"
    container_port   = 8081
  }
}

# ── Outputs ───────────────────────────────────────────────
output "alb_dns" {
  value = aws_lb.main.dns_name
}