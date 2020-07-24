import * as cdk from '@aws-cdk/core';
import * as events from '@aws-cdk/aws-events';
import * as iam from '@aws-cdk/aws-iam';
import * as lambda from '@aws-cdk/aws-lambda';
import * as secretsmanager from '@aws-cdk/aws-secretsmanager';
import * as targets from '@aws-cdk/aws-events-targets';

import { readFileSync } from 'fs';

export interface SsoSyncStackProps  extends cdk.StackProps {
  /**
   * The location of the Google credentials.json
   */
  readonly googleCredendtialsFileLocation: string;

  /**
   * The location of the Google token.json
   */
  readonly googleTokenFileLocation: string;

  /**
   * The location of the AWS aws.toml file for SCIM
   */
  readonly awsTomlFileLocation: string;

  /**
   * The path of the linux binary of ssosync
   */
  readonly lambdaPath: string;
}

export class CdkStack extends cdk.Stack {
  constructor(scope: cdk.Construct, id: string, props: SsoSyncStackProps) {
    super(scope, id, props);

    const googleCredentials = readFileSync(props.googleCredendtialsFileLocation, 'utf8');
    const googleTokens = readFileSync(props.googleTokenFileLocation, 'utf8');
    const awsToml = readFileSync(props.awsTomlFileLocation, 'utf8');

    const googleCredSecret = new secretsmanager.CfnSecret(this, 'GoogleCred', {
      secretString: googleCredentials,
    });

    const googleTokenSecret  = new secretsmanager.CfnSecret(this, 'GoogleToken', {
      secretString: googleTokens,

    });

    const awsTomlSecret  = new secretsmanager.CfnSecret(this, 'AwsToml', {
      secretString: awsToml,
    });

    const lambdaFn = new lambda.Function(this, 'SsoSync', {
      code: new lambda.AssetCode(props.lambdaPath),
      runtime: lambda.Runtime.GO_1_X,
      handler: 'ssosync',
      timeout: cdk.Duration.seconds(30),
      environment: {
        "SSOSYNC_GOOGLE_CREDENTIALS": googleCredSecret.ref,
        "SSOSYNC_GOOGLE_TOKEN": googleTokenSecret.ref,
        "SSOSYNC_AWS_TOML": awsTomlSecret.ref,
      }
    })

    lambdaFn.role?.addToPrincipalPolicy(new iam.PolicyStatement({
      actions: ['secretsmanager:GetSecretValue'],
      resources: [
        googleCredSecret.ref,
        googleTokenSecret.ref,
        awsTomlSecret.ref,
      ],
    }))

    const rule = new events.Rule(this, 'Rule', {
      schedule: events.Schedule.expression('rate(1 hour)')
    });

    rule.addTarget(new targets.LambdaFunction(lambdaFn));
  }
}
