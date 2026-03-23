variable "db_password" {
  description = "RDS MySQL password"
  type        = string
  sensitive   = true
  default     = "Password123!"
}

variable "db_backend" {
  description = "Which DB backend: mysql or dynamodb"
  type        = string
  default     = "mysql"
  validation {
    condition     = contains(["mysql", "dynamodb"], var.db_backend)
    error_message = "db_backend must be mysql or dynamodb"
  }
}