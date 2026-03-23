output "alb_url" {
  value = "http://${aws_lb.main.dns_name}"
}

output "ecr_url" {
  value = aws_ecr_repository.app.repository_url
}

output "mysql_host" {
  value = aws_db_instance.mysql.address
}

output "dynamodb_table" {
  value = aws_dynamodb_table.carts.name
}