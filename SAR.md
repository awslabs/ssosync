# SSO Sync

This AWS Serverless Application populates AWS SSO directly with your Google Apps users.

## Setup

Before you start, you have to [enable AWS SSO](https://docs.aws.amazon.com/singlesignon/latest/userguide/step1.html) in AWS Organizations. The next steps are to configure the access to the Google APIs and the AWS SSO SCIM endpoint.

### Google

First, you have to setup your API. In the project you want to use go to the [Console](https://console.developers.google.com/apis) and select *API & Services* > *Enable APIs and Services*. Search for *Admin SDK* and *Enable* the API. 

You have to perform this [tutorial](https://developers.google.com/admin-sdk/directory/v1/guides/delegation) to create a service account that you use to sync your users. Save the JSON file you create during the process and rename it to `credentials.json`. 

In the domain-wide delegation for the Admin API, you have to specificy the following scopes for the user.

`https://www.googleapis.com/auth/admin.directory.group.readonly,https://www.googleapis.com/auth/admin.directory.group.member.readonly,https://www.googleapis.com/auth/admin.directory.user.readonly`

Back in the Console go to the Dashboard for the API & Services and select "Enable API and Services".
In the Search box type `Admin` and select the `Admin SDK` option. Click the `Enable` button.

There are general configuration parameters to the application stack.

* `GoogleCredentials` contains the content of the `credentials.json` file
* `GoogleAdminEmail` contains the email address of an admin

The secrets are stored in the [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/).

### AWS

Go to the AWS Single Sign-On console in the region you have set up AWS SSO and select
Settings. Click `Enable automatic provisioning`.

A pop up will appear with URL and the Access Token. The Access Token will only appear
at this stage. You want to copy both of these to the stack parameters.

* `SCIMEndpointUrl`
* `SCIMEndpointAccessToken`

You are ready to either to deploy the application to your account.
