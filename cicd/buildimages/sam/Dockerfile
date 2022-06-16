FROM aws/codebuild/standard:5.0 AS build
# Update sam to latest version
RUN wget -q https://github.com/aws/aws-sam-cli/releases/latest/download/aws-sam-cli-linux-x86_64.zip \
 && unzip -q aws-sam-cli-linux-x86_64.zip -d sam-installation \
 && sudo ./sam-installation/install --update \
 && rm -rf ./sam-installation  aws-sam-cli-linux-x86_64.zip 

