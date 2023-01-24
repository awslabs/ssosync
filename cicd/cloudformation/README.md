# SSO Sync Pipelines

There are a number of cloudformation templates depending on what your need to deploy. 
Google Environment 
If you new to ssosync, start with the 

For most developers,You need
* secrets.yaml - creates the secrets for storing the credentials for your test GSuite and IAM Identity Center instances
* developer.yaml - creates the pipeline to build and test prior to raising a pull request.

The other option is for the production build, deploy and test environment, which requires, two AWS accounts *production* and *staging*:
* Create the management account
  * setup organizations
  * create two linked accounts (delegated & non-delegated respectively)
  * setup IAM Identity Center
  * delegate administration to the *delegated* account

* Google Environment
  * A quick way to get setup is using the [AWS Control Tower Workshop](https://catalog.workshops.aws/control-tower) : [Google Workspace Lab](https://catalog.workshops.aws/control-tower/en-US/authentication-authorization/google-workspace)
  * Setup [GAM](https://github.com/GAM-team/GAM/wiki/#introduction) 
  * [ Coming soon ] Run the Directory Prep Scripts to setup the users and groups

  You'll need to follow the instructions in the main README to get the api access setup
You'll need to validate the domain you register for it.

Full instructions are available in the 

* Deploy the following stacks into each *staging* account (management, delegated IAM Identity Center admin, non-delegated)
  * secrets.yaml - creates the secrets for storing the credentials for your test GSuite and IAM Identity Center instances
  * testing.yaml - creates the pipeline to deploy and test prior to raising a pull request.
Make a note of the output values

* Now setup your *production* account
  * Manually create your code star connection
  * release.yaml - creates the pipeline to build, trigger the test pipeline in staging and where appropriate publish the app


