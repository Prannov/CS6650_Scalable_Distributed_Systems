terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # S3 backend to persist state across Academy sessions
  backend "s3" {
    bucket = "sportspulse-tf-state-637423359726"
    key    = "sportspulse/terraform.tfstate"
    region = "us-east-1"
  }
}

provider "aws" {
  region = "us-east-1"
}