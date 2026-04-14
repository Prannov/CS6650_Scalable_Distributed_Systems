# ── S3 Bucket ─────────────────────────────────────────────────────────────────
resource "aws_s3_bucket" "photos" {
  bucket        = "${var.app_name}-photos-${data.aws_caller_identity.current.account_id}"
  force_destroy = true
  tags          = { Name = "${var.app_name}-photos" }
}

resource "aws_s3_bucket_public_access_block" "photos" {
  bucket = aws_s3_bucket.photos.id

  # Allow public access so presigned URLs work without extra config
  block_public_acls       = false
  block_public_policy     = false
  ignore_public_acls      = false
  restrict_public_buckets = false
}

resource "aws_s3_bucket_cors_configuration" "photos" {
  bucket = aws_s3_bucket.photos.id

  cors_rule {
    allowed_methods = ["GET", "PUT", "POST"]
    allowed_origins = ["*"]
    allowed_headers = ["*"]
    max_age_seconds = 3000
  }
}

data "aws_caller_identity" "current" {}

# ── SQS Queue ─────────────────────────────────────────────────────────────────
resource "aws_sqs_queue" "photo_queue" {
  name                       = "${var.app_name}-photo-queue"
  visibility_timeout_seconds = 60   # must process within 60s or message re-appears
  message_retention_seconds  = 3600 # 1 hour
  receive_wait_time_seconds  = 20   # long polling

  tags = { Name = "${var.app_name}-photo-queue" }
}

# Dead-letter queue — catches messages that fail repeatedly
resource "aws_sqs_queue" "photo_dlq" {
  name                      = "${var.app_name}-photo-dlq"
  message_retention_seconds = 86400
}

resource "aws_sqs_queue_redrive_policy" "photo_queue" {
  queue_url = aws_sqs_queue.photo_queue.id
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.photo_dlq.arn
    maxReceiveCount     = 3
  })
}
