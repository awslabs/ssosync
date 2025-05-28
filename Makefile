OUTPUT = main # Referenced as Handler in sar-template.yaml
RELEASER = goreleaser
PACKAGED_TEMPLATE = packaged.yaml
STACK_NAME := $(STACK_NAME)
S3_BUCKET := $(S3_BUCKET)
S3_PREFIX := $(S3_PREFIX)
TEMPLATE = sar-template.yaml
APP_NAME ?= ssosync
GOREL ?= go run github.com/goreleaser/goreleaser/v2@latest

.PHONY: test
test:
	rm internal/mocks/* -f
	mockery
	go test ./... -coverprofile=coverage.out

.PHONY: go-build
go-build:
	go build -o $(APP_NAME) main.go

.PHONY: clean
clean:
	rm -f $(OUTPUT) $(PACKAGED_TEMPLATE)

build-SSOSyncFunction:
	GOOS=linux GOARCH=arm64 go build -o bootstrap main.go
	cp dist/ssosync_linux_arm64/ssosync $(ARTIFACTS_DIR)/bootstrap

.PHONY: install
install:
	go get ./...

main: main.go
	$(GOREL) build --snapshot --clean

# compile the code to run in Lambda (local or real)
.PHONY: lambda
lambda:
	$(MAKE) main

.PHONY: build
build: clean lambda

.PHONY: api
api: build
	sam local start-api

.PHONY: publish
publish:
	sam publish -t packaged.yaml

.PHONY: package
package: build
	cp dist/ssosync_linux_arm64/ssosync ./bootstrap
	sam package --s3-bucket $(S3_BUCKET) --output-template-file $(PACKAGED_TEMPLATE) --s3-prefix $(S3_PREFIX)

.PHONY: deploy
deploy: package
	sam deploy --stack-name $(STACK_NAME) --template-file $(PACKAGED_TEMPLATE) --capabilities CAPABILITY_IAM
