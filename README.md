# SSO Sync

> Helping you populate AWS SSO directly with your Google Apps users

SSO Sync will run on any platform that Go can build for.

## Why?

As per the [AWS SSO](https://aws.amazon.com/single-sign-on/) Homepage:

> AWS Single Sign-On (SSO) makes it easy to centrally manage access
> to multiple AWS accounts and business applications and provide users
> with single sign-on access to all their assigned accounts and applications
> from one place.

Key part further down:

> With AWS SSO, you can create and manage user identities in AWS SSO’s
>identity store, or easily connect to your existing identity source including
> Microsoft Active Directory and **Azure Active Directory (Azure AD)**.

AWS SSO can use other Identity Providers as well... such as Google Apps for Domains. Although AWS SSO
supports a subset of the SCIM protocol for populating users, it currently only has support for Azure AD.

This project provides a CLI tool to pull users and groups from Google and push them into AWS SSO.
`ssosync` deals with removing users as well. The heavily commented code provides you with the detail of
what it is going to do.

### References

 * [SCIM Protocol RFC](https://tools.ietf.org/html/rfc7644)
 * [AWS SSO - Connect to Your External Identity Provider](https://docs.aws.amazon.com/singlesignon/latest/userguide/manage-your-identity-source-idp.html)
 * [AWS SSO - Automatic Provisioning](https://docs.aws.amazon.com/singlesignon/latest/userguide/provision-automatically.html)

## Installation

You can `go get github.com/awslabs/ssosync` or grab a Release binary from the release page.

## Configuration

You need a few items of configuration. One side from AWS, and the other
from Google Cloud / Apps to allow for API access to each. You should have configured
Google as your Identity Provider for AWS SSO already.

### Google

Head to the [Google Cloud Console](https://console.cloud.google.com/) for your Domain
(Specifically API & Services ->
[Credentials](https://console.cloud.google.com/projectselector2/apis/credentials))
and Create a Project.

Creating a project will take a few seconds. Once it is complete, you can then Configure the Consent
Screen (there will be a clear warning and button for it). Click Through and select "Internal". Give
a name and press Save as you don't need the rest.

Now go back to Credentials, Click Create Credentials and then select OAuth client ID. Select Other and
provide a name. You will be displayed credentials, press okay and then use the download button, and a
JSON file will download.

**THIS FILE IS IMPORTANT AND SECRET - KEEP IT SAFE**

With this done, you can log in and generate a token.json file. To create the file, use the
`ssosync google` command. With help output, it looks like this:

```text
Log in to Google - use me to generate the files needed for the main command

Usage:
  ssosync google [flags]

Flags:
  -h, --help               help for google
      --path string        set the path to find credentials (default "credentials.json")
      --tokenPath string   set the path to put token.json output into (default "token.json")
```

When you run the command correctly, it will give a URL to load in your browser. Go to it, and you'll get
a string to paste back and enter. Once you paste the line in, the file generates.

The Token file is useless without the Credentials File - but keep it safe.

Back in the Console go to the Dashboard for the API & Services and select "Enable API and Services".
In the Search box type `Admin` and select the `Admin SDK` option. Click the `Enable` button.

### AWS

Go to the AWS Single Sign-On console in the region you have set up AWS SSO and select
Settings. Click `Enable automatic provisioning`.

A pop up will appear with URL and the Access Token. The Access Token will only appear
at this stage. You want to copy both of these into a text file which ends in the extension
`.toml`.

```toml
Token    = "tokenHere"
Endpoint = "https://scim.eu-west-1.amazonaws.com/a-guid-would-be-here/scim/v2/"
```

##

Usage:

The default for ssosync is to run through the sync. process

```text
A command line tool to enable you to synchronise your Google
Apps (G-Suite) users to AWS Single Sign-on (AWS SSO)

Usage:
  ssosync [flags]
  ssosync [command]

Available Commands:
  google      Log in to Google
  help        Help about any command

Flags:
  -d, --debug                          Enable verbose / debug logging
  -c, --googleCredentialsPath string   set the path to find credentials for Google (default "credentials.json")
  -t, --googleTokenPath string         set the path to find token for Google (default "token.json")
  -h, --help                           help for ssosync
  -s, --scimConfig string              AWS SSO SCIM Configuration (default "aws.toml")

Use "ssosync [command] --help" for more information about a command.
```

The output of the command when run without 'debug' turned on looks like this:

```
2020-05-26T12:08:14.083+0100	INFO	cmd/root.go:43	Creating the Google and AWS Clients needed
2020-05-26T12:08:14.084+0100	INFO	internal/sync.go:38	Start user sync
2020-05-26T12:08:14.979+0100	INFO	internal/sync.go:73	Clean up AWS Users
2020-05-26T12:08:14.979+0100	INFO	internal/sync.go:89	Start group sync
2020-05-26T12:08:15.578+0100	INFO	internal/sync.go:135	Start group user sync	{"group": "AWS Administrators"}
2020-05-26T12:08:15.703+0100	INFO	internal/sync.go:172	Clean up AWS groups
2020-05-26T12:08:15.703+0100	INFO	internal/sync.go:183	Done sync groups
```
