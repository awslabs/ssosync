data "template_file" "container_definitions_file" {
  template = file("./container_definitions.json.tmpl")

  vars = {
      name                = local.tags.Name
      environment         = local.tags.Environment
      ecr_repository      = aws_ecr_repository.this.repository_url
      region              = data.aws_region.current.name
      application_ssm_arn = "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter/${local.tags.Name}/${local.tags.Environment}"
    }
}

resource "aws_ecs_task_definition" "this" {
  family                = local.tags.Name
  container_definitions = data.template_file.container_definitions_file.rendered

  execution_role_arn = aws_iam_role.task_execution_role.arn
  task_role_arn      = aws_iam_role.task_role.arn

  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  network_mode             = "awsvpc"


  tags = merge(local.tags)
}

