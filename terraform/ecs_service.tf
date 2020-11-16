data "aws_iam_policy_document" "service_assume_role" {
  statement {
    effect = "Allow"

    principals {
      type = "Service"
      identifiers = [
        "ecs.amazonaws.com",
      ]
    }

    actions = ["sts:AssumeRole"]
  }
}

data "aws_iam_policy_document" "service_policy" {
  statement {
    effect = "Allow"

    resources = ["*"]

    actions = ["logs:*"]
  }

  statement {
    effect = "Allow"

    resources = ["*"]

    actions = [
      "ecs:CreateCluster",
      "ecs:DeregisterContainerInstance",
      "ecs:DiscoverPollEndpoint",
      "ecs:Poll",
      "ecs:RegisterContainerInstance",
      "ecs:StartTelemetrySession",
      "ecs:UpdateContainerInstancesState",
      "ecs:Submit*",
      "ecr:GetAuthorizationToken",
      "ecr:BatchCheckLayerAvailability",
      "ecr:GetDownloadUrlForLayer",
      "ecr:BatchGetImage",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
      "ec2:Describe*",
    ]
  }
}


resource "aws_iam_role" "service_role" {
  name               = "${local.tags.Name}-service"
  path               = "/"
  assume_role_policy = data.aws_iam_policy_document.service_assume_role.json

  tags = merge(local.tags)
}

resource "aws_iam_role_policy" "service_policy" {
  name   = "${local.tags.Name}-service"
  policy = data.aws_iam_policy_document.service_policy.json
  role   = aws_iam_role.service_role.name
}

resource "aws_ecs_service" "this" {

  name                               = local.tags.Name
  cluster                            = aws_ecs_cluster.this.id
  task_definition                    = aws_ecs_task_definition.this.arn
  desired_count                      = 1
  launch_type                        = "FARGATE"

  network_configuration {
    subnets = data.aws_subnet_ids.private.ids
  }

}

