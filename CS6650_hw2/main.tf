############################################
# Variables
############################################

variable "ssh_cidr" {
  type        = string
  description = "Your home IP in CIDR notation"
}

variable "ssh_key_name" {
  type        = string
  description = "Name of your existing AWS key pair"
}

############################################
# Provider
############################################

provider "aws" {
  region = "us-east-1"
}

############################################
# Data sources
############################################

# Latest Amazon Linux 2023 AMI
data "aws_ami" "al2023" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-*-x86_64-ebs"]
  }
}

# EXISTING working instance (from your screenshot)
data "aws_instance" "existing" {
  instance_id = "i-0923754db7efcc671"
}

############################################
# EC2 Instance (reuse existing SGs)
############################################

resource "aws_instance" "demo-instance" {
  ami                  = data.aws_ami.al2023.id
  instance_type        = "t2.micro"
  iam_instance_profile = "LabInstanceProfile"
  key_name             = var.ssh_key_name

  # IMPORTANT: reuse SGs from working instance
  vpc_security_group_ids = data.aws_instance.existing.vpc_security_group_ids

  tags = {
    Name = "terraform-created-instance-:)"
  }
}

############################################
# Outputs
############################################

output "ec2_public_dns" {
  value = aws_instance.demo-instance.public_dns
}
