# SSO Sync Pipelines

There are a number of cloudformation templates depending on what your need to deploy. For most developers 
You need 
* secrets.yaml - creates the secrets for storing the credentials for your test GSuite and IAM Identity Center instances
* developer.yaml - creates the pipeline to build and test prior to raising a pull request.

The other option is for the production build, deploy and test environment, which requires, two AWS accounts *production* and *staging*:
* Create the management account
  * setup organizations
  * create two linked accounts (delegated & non-delegated respectively)
  * setup IAM Identity Center
  * delegate administration to the *delegated* account

* Deploy the following stacks into each *staging* account (management, delegated IAM Identity Center admin, non-delegated)
  * secrets.yaml - creates the secrets for storing the credentials for your test GSuite and IAM Identity Center instances
  * testing.yaml - creates the pipeline to deploy and test prior to raising a pull request.
Make a note of the output values

* Now setup your *production* account
  * Manually create your code star connection
  * release.yaml - creates the pipeline to build, trigger the test pipeline in staging and where appropriate publish the app


