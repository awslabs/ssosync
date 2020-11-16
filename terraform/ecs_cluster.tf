resource "aws_ecs_cluster" "this" {
  name = local.tags.Name

  capacity_providers = ["FARGATE"]

  setting {
    name  = "containerInsights"
    value = "enabled"
    }

  tags = merge(local.tags)
}
