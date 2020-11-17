resource "aws_cloudwatch_log_group" "cloudwatch_log_group" {
  name = "/ecs/${var.name}"

  tags = merge(local.tags)
}

resource "aws_cloudwatch_log_stream" "cloudwatch_log_stream" {
  name           = var.name
  log_group_name = aws_cloudwatch_log_group.cloudwatch_log_group.name
}

