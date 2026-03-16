# variables.tf
variable "aws_region" {
  default = "us-east-1"
}
variable "ecr_repo_receiver" {
  description = "ECR image URI for order-receiver"
}
variable "ecr_repo_processor" {
  description = "ECR image URI for order-processor"
}
variable "num_workers" {
  description = "Number of goroutine workers in the processor"
  default     = 1
}