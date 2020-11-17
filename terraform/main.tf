module "standard_application_fargate" {
  source = "./modules/standard_application_fargate"

  name        = var.name
  application = var.application
  owner       = var.owner
  environment = var.environment
  squad       = var.squad

  container_definitions = var.container_definitions
}
