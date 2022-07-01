# Copyright 2020-2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Amazon Software License (the "License"). You may not use this file except in compliance with the License.
# A copy of the License is located at
#
#    http://aws.amazon.com/asl/
#
# or in the "license" file accompanying this file.
# This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, express or implied.
# See the License for the specific language governing permissions and limitations under the License.
# 
# Based on https://github.com/aws/aws-codebuild-docker-images/blob/master/ubuntu/standard/5.0/Dockerfile
# and updated to reflect the build requirements of this project
#
FROM public.ecr.aws/codebuild/standard:5.0 AS build

FROM build AS sam

# Install SAM CLI to latest version
# https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install-linux.html
RUN wget -q https://github.com/aws/aws-sam-cli/releases/latest/download/aws-sam-cli-linux-x86_64.zip \
    && unzip -q aws-sam-cli-linux-x86_64.zip -d sam-installation \
    && sudo ./sam-installation/install --update \
    && rm -rf ./sam-installation  aws-sam-cli-linux-x86_64.zip

# Install env tools for runtimes
#go
RUN git clone https://github.com/syndbg/goenv.git $HOME/.goenv
ENV PATH="/root/.goenv/shims:/root/.goenv/bin:/go/bin:$PATH"
ENV GOENV_DISABLE_GOPATH=1
ENV GOPROXY=direct
ENV GOPATH="/go"

#=======================End of layer: tools  =================
FROM sam AS runtimes

#****************     GOLANG     ****************************************************
ENV GOLANG_18_VERSION="1.18.2"

RUN goenv install $GOLANG_18_VERSION && rm -rf /tmp/* && \
    goenv global  $GOLANG_18_VERSION

RUN go get -u github.com/golang/dep/cmd/dep

# Install golint
RUN go install golang.org/x/lint/golint@latest

# Install staticcheck
RUN go install honnef.co/go/tools/cmd/staticcheck@latest

# Install Testify to use common assertions and mocks in tests
RUN go get -u github.com/stretchr/testify

# Install goreleaser
RUN go install github.com/goreleaser/goreleaser@latest

#****************      END GOLANG     *******************************

#=======================End of layer: runtimes  =================

FROM public.ecr.aws/ubuntu/standard:5.0
# Configure SSH
COPY /root/.ssh/config /root/.ssh/config
COPY /codebuild/image/config/runtimes.yml /codebuild/image/config/runtimes.yml
COPY /usr/local/bin/dockerd-entrypoint.sh /usr/local/bin/dockerd-entrypoint.sh
COPY /usr/share/doc/bill_of_material.txt     /usr/share/doc/bill_of_material.txt
COPY /etc/amazon/ssm/amazon-ssm-agent.json          /etc/amazon/ssm/amazon-ssm-agent.json

ENTRYPOINT ["/usr/local/bin/dockerd-entrypoint.sh"]
