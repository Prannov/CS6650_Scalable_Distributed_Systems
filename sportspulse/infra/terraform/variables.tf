variable "region" {
  default = "us-east-1"
}

variable "account_id" {
  default = "637423359726"
}

variable "db_password" {
  default   = "SportsPulse2024!"
  sensitive = true
}

variable "query_svc_count" {
  description = "Number of query-svc ECS tasks (1, 2, 4, or 8 for Experiment 3)"
  default     = 1
}