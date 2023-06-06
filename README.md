# SSO Sync

Fork of https://github.com/awslabs/ssosync.

## Deploy Changes

### Prerequisites

- AWS CLI.
- SAM CLI.

### Steps

1. Sign into the zn-master account with your AWS CLI.
2. Run `make package` to build and upload the build artifact.
3. Update the `ssosyncv2` CloudFormation stack in the zn-master account using
   the template file `packaged.yaml` on your local machine.
