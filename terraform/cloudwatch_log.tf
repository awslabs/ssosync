resource "aws_cloudwatch_log_group" "cloudwatch_log_group" {
  name = "/ecs/${local.tags.Name}"

  tags = merge(local.tags)
}

resource "aws_cloudwatch_log_stream" "cloudwatch_log_stream" {
  name           = local.tags.Name
  log_group_name = aws_cloudwatch_log_group.cloudwatch_log_group.name
}

