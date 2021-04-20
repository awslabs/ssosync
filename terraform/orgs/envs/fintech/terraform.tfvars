region = "us-east-1"
name = "ssosync"
application = "ssosync"
environment = "shared-services"
squad = "ProductionEngineering"
owner = "camila.avila"

container_definitions = "./templates/container_definitions.json.tmpl"
include_groups = "access-aws-fintech@creditas.com,aws-creditas-dev@creditas.com,aws-creditas-de@creditas.com,aws-creditas-admin@creditas.com,aws-creditas-terceiro-tivit@creditas.com,aws-creditas-dbe@creditas.com,aws-creditas-bi@creditas.com,aws-creditas-dp@creditas.com,aws-creditas-adp@creditas.com,aws-creditas-ds@creditas.com,aws-creditas-mle@creditas.com,aws-creditas-terceiro-bitone@creditas.com,aws-creditas-terceiro-cesar-til@creditas.com,aws-creditas-terceiro-chp@creditas.com,aws-creditas-terceiro-cobmais@creditas.com,aws-creditas-terceiro-empirica@creditas.com,aws-creditas-terceiro-fidic-anga@creditas.com,aws-creditas-funding@creditas.com,aws-creditas-terceiro-gaia@creditas.com,aws-creditas-terceiro-oliveira-trust@creditas.com,aws-creditas-infra@creditas.com,aws-creditas-procurement@creditas.com,aws-creditas-ph@creditas.com,aws-creditas-mexico@creditas.com,aws-creditas-people-analytics@creditas.com,aws-creditas-legacy-bankfacil-apps@creditas.com,aws-creditas-consignado@creditas.com,aws-creditas-pe@creditas.com,aws-creditas-boc@creditas.com,aws-creditas-sd@creditas.com"

