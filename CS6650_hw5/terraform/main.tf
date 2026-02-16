locals {
  common_tags = {
    Project     = var.project_name
    Environment = var.environment
    ManagedBy   = "Terraform"
  }
}

# Get available AZs
data "aws_availability_zones" "available" {
  state = "available"
}

# ECR Module
module "ecr" {
  source = "./modules/ecr"
  
  repository_name = var.project_name
  tags            = local.common_tags
}

# Networking Module
module "networking" {
  source = "./modules/networking"
  
  project_name       = var.project_name
  vpc_cidr           = "10.0.0.0/16"
  availability_zones = slice(data.aws_availability_zones.available.names, 0, 2)
  container_port     = var.container_port
  tags               = local.common_tags
}

# ECS Module
module "ecs" {
  source = "./modules/ecs"
  
  project_name           = var.project_name
  cluster_name           = "${var.project_name}-cluster"
  ecr_repository_url     = module.ecr.repository_url
  container_port         = var.container_port
  cpu                    = var.cpu
  memory                 = var.memory
  desired_count          = var.desired_count
  vpc_id                 = module.networking.vpc_id
  public_subnet_ids      = module.networking.public_subnet_ids
  alb_security_group_id  = module.networking.alb_security_group_id
  ecs_security_group_id  = module.networking.ecs_security_group_id
  aws_region             = var.aws_region
  tags                   = local.common_tags
  
  depends_on = [module.ecr, module.networking]
}