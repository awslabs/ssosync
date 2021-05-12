region = "us-east-1"
name = "ssosync"
application = "ssosync"
environment = "shared-services"
squad = "ProductionEngineering"
owner = "rafael.leonardo"

container_definitions = "./templates/container_definitions.json.tmpl"

include_groups = [
    "aws-scd-admin@creditas.com",
    "aws-scd-dev@creditas.com",
    "aws-scd-infra@creditas.com",
    "aws-scd-de@creditas.com",
    "aws-scd-security@creditas.com",
    "aws-scd-procurement@creditas.com",
    "aws-scd-dbe@creditas.com",
    "aws-scd-pe@creditas.com",
    "aws-scd-core@creditas.com",
    "aws-scd-core-admin@creditas.com",
    "aws-scd-principals@creditas.com",
]
