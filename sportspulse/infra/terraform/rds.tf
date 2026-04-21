resource "aws_db_subnet_group" "main" {
  name       = "sportspulse-db-subnet"
  subnet_ids = [aws_subnet.public_a.id, aws_subnet.public_b.id]
  tags       = { Name = "sportspulse-db-subnet" }
}

resource "aws_db_instance" "postgres" {
  identifier           = "sportspulse-db"
  engine               = "postgres"
  engine_version       = "16"
  instance_class       = "db.t3.micro"
  allocated_storage    = 20
  db_name              = "sportspulse"
  username             = "sp_user"
  password             = var.db_password
  db_subnet_group_name = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  skip_final_snapshot  = true
  publicly_accessible  = true
  tags                 = { Name = "sportspulse-db" }
}

output "db_endpoint" {
  value = aws_db_instance.postgres.endpoint
}