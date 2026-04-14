# AWS Academy accounts cannot create IAM roles — use the pre-existing LabRole
data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

# CloudWatch Log Group
resource "aws_cloudwatch_log_group" "app" {
  name              = "/ecs/${var.app_name}"
  retention_in_days = 7
}
