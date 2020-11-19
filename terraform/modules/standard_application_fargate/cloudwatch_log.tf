resource "aws_cloudwatch_log_group" "this" {
  name = "/ecs/${var.name}"

  tags = merge(local.tags)
}

resource "aws_cloudwatch_log_stream" "this" {
  name           = var.name
  log_group_name = aws_cloudwatch_log_group.this.name
}

