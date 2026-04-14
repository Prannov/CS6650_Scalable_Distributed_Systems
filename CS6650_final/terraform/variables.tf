variable "aws_region" {
  default = "us-east-1"
}

variable "app_name" {
  default = "album-store"
}

variable "db_password" {
  description = "RDS master password"
  sensitive   = true
}

variable "api_image" {
  description = "ECR image URI for API, e.g. 123456789.dkr.ecr.us-east-1.amazonaws.com/album-store-api:latest"
}

variable "worker_image" {
  description = "ECR image URI for worker"
}

variable "api_count" {
  description = "Number of API ECS tasks"
  default     = 2
}

variable "worker_count" {
  description = "Number of worker ECS tasks"
  default     = 2
}

variable "api_cpu" {
  default = 1024
}

variable "api_memory" {
  default = 2048
}

variable "worker_cpu" {
  default = 512
}

variable "worker_memory" {
  default = 1024
}
