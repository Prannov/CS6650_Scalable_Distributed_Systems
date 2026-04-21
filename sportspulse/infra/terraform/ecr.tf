resource "aws_ecr_repository" "event_svc" {
  name                 = "sportspulse-event-svc"
  image_tag_mutability = "MUTABLE"
  force_delete         = true
}

resource "aws_ecr_repository" "stats_worker" {
  name                 = "sportspulse-stats-worker"
  image_tag_mutability = "MUTABLE"
  force_delete         = true
}

resource "aws_ecr_repository" "query_svc" {
  name                 = "sportspulse-query-svc"
  image_tag_mutability = "MUTABLE"
  force_delete         = true
}

output "ecr_event_svc_url" {
  value = aws_ecr_repository.event_svc.repository_url
}

output "ecr_stats_worker_url" {
  value = aws_ecr_repository.stats_worker.repository_url
}

output "ecr_query_svc_url" {
  value = aws_ecr_repository.query_svc.repository_url
}