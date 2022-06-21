#!/bin/bash

export CI=true
export CodeBuild=true

export GitBranch=`git symbolic-ref HEAD --short 2>/dev/null`
if [ "$GitBranch" == "" ] ; then
  GitBranch=`git branch -a --contains HEAD | sed -n 2p | awk '{ printf $1 }'`
  export GitBranch=${GitBranch#remotes/origin/}
fi

export GitMessage=`git log -1 --pretty=%B`
export GitAuthor=`git log -1 --pretty=%an`
export GitAuthorEmail=`git log -1 --pretty=%ae`
export GitCommit=`git log -1 --pretty=%H`
export GITTag=`git describe --tags --abbrev=0`

export GitPullRequest=false
if [[ $GitBranch == pr-* ]] ; then
  export GitPullRequest=${GitBranch#pr-}
fi

export GitProject=${APP_NAME}
export CodeBuildUrl=https://$AWS_DEFAULT_REGION.console.aws.amazon.com/codebuild/home?region=$AWS_DEFAULT_REGION#/builds/$CODEBUILD_BUILD_ID/view/new

echo "==> AWS CodeBuild Extra Environment Variables:"
echo "==> CI = $CI"
echo "==> CodeBuild = $CodeBuild"
echo "==> GitAuthor = $GitAuthor"
echo "==> GitAuthorEmail = $GitAuthorEmail"
echo "==> GitBranch = $GitBranch "
echo "==> GitCommit = $GitCommit"
echo "==> GitMessage = $GitMessage"
echo "==> GitTag = $GITTag"
echo "==> GitProject = $GitProject"
echo "==> GitPullRequest = $GitPullRequest"
