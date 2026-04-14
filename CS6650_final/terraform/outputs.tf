output "alb_dns_name" {
  description = "Your service's public URL — use this as base_url in ChaosArena submission"
  value       = "http://${aws_lb.main.dns_name}"
}

output "ecr_api_url" {
  description = "Push your API image here"
  value       = aws_ecr_repository.api.repository_url
}

output "ecr_worker_url" {
  description = "Push your worker image here"
  value       = aws_ecr_repository.worker.repository_url
}

output "s3_bucket" {
  value = aws_s3_bucket.photos.bucket
}

output "sqs_queue_url" {
  value = aws_sqs_queue.photo_queue.url
}

output "rds_endpoint" {
  value = aws_db_instance.postgres.endpoint
}

output "redis_endpoint" {
  value = "${aws_elasticache_cluster.redis.cache_nodes[0].address}:6379"
}
