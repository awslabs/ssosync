#!/usr/bin/env node
import 'source-map-support/register';
import * as cdk from '@aws-cdk/core';
import { CdkStack } from '../lib/cdk-stack';

const app = new cdk.App();

new CdkStack(app, 'SsoSyncStack', {
  awsTomlFileLocation: process.env.AWS_TOML || "../../aws.toml",
  googleCredendtialsFileLocation: process.env.GOOGLE_CREDENTIALS || "../../credentials.json",
  googleTokenFileLocation: process.env.GOOGLE_TOKEN || "../../token.json",
  lambdaPath: process.env.SSOSYNC_PATH || "../../dist/ssosync_linux_amd64"
});
