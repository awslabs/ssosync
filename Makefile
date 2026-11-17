OUTPUT = main # Referenced as Handler in sar-template.json

PACKAGED_TEMPLATE = packaged.yaml
STACK_NAME := $(STACK_NAME)
S3_BUCKET := $(S3_BUCKET)
S3_PREFIX := $(S3_PREFIX)
TEMPLATE = template.yaml
APP_NAME ?= ssosync
GOREL_ARGS ?= 
GOREL ?= goreleaser

# Verbose flag support
# Use VERBOSE=1 for verbose output (e.g., 'make test VERBOSE=1')
VERBOSE ?= 0
ifeq ($(VERBOSE),1)
	VERBOSE_FLAG = -v
	VERBOSE_TEST_FLAG = -v
	VERBOSE_BUILD_FLAG = --verbose
	VERBOSE_CURL_FLAG = -v
	Q =
else
	VERBOSE_FLAG =
	VERBOSE_TEST_FLAG =
	VERBOSE_BUILD_FLAG =
	VERBOSE_CURL_FLAG = -s
	Q = @
endif

# Tool versions
MOCKERY_VERSION ?= v3.8.1
GOLANGCI_LINT_VERSION ?= v2.15.0
GORELEASER_VERSION ?= v2.17.0
UPX_VERSION ?= v4.2.4

# Tool installation paths
TOOLS_DIR := $(shell pwd)/.bin
MOCKERY := $(TOOLS_DIR)/mockery
GOLANGCI_LINT := $(TOOLS_DIR)/golangci-lint
GORELEASER_BIN := $(TOOLS_DIR)/goreleaser
UPX := $(TOOLS_DIR)/upx

# Detect OS and architecture
OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)
ifeq ($(ARCH),x86_64)
	ARCH := x86_64

endif
ifeq ($(ARCH),aarch64)
	ARCH := arm64
endif

.PHONY: install-deps
install-deps: install-mockery install-golangci-lint install-goreleaser install-upx
	$(Q)echo "All development dependencies installed"

.PHONY: install-mockery
install-mockery:
	$(Q)echo "Installing mockery $(MOCKERY_VERSION)..."
	$(Q)mkdir -p $(TOOLS_DIR)
	@if [ ! -f $(MOCKERY) ] || [ "$$($(MOCKERY) version 2>/dev/null | grep -o 'v[0-9]\+\.[0-9]\+\.[0-9]\+')" != "$(MOCKERY_VERSION)" ]; then \
		MOCKERY_ARCH=$(ARCH); \
		MOCKERY_OS=$(OS); \
		if [ "$(ARCH)" = "amd64" ]; then MOCKERY_ARCH="x86_64"; fi; \
		if [ "$(OS)" = "darwin" ]; then MOCKERY_OS="Darwin"; fi; \
		if [ "$(OS)" = "linux" ]; then MOCKERY_OS="Linux"; fi; \
		curl $(VERBOSE_CURL_FLAG)SfL https://github.com/vektra/mockery/releases/download/$(MOCKERY_VERSION)/mockery_$(MOCKERY_VERSION:v%=%)_$${MOCKERY_OS}_$${MOCKERY_ARCH}.tar.gz | tar -xz -C $(TOOLS_DIR) mockery; \
		chmod +x $(MOCKERY); \
		echo "mockery $(MOCKERY_VERSION) installed"; \
	else \
		echo "mockery $(MOCKERY_VERSION) already installed"; \
	fi

.PHONY: install-golangci-lint
install-golangci-lint:
	$(Q)echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	$(Q)mkdir -p $(TOOLS_DIR)
	@if [ ! -f $(GOLANGCI_LINT) ] || [ "$$($(GOLANGCI_LINT) --version 2>/dev/null | grep -o 'v[0-9]\+\.[0-9]\+\.[0-9]\+')" != "$(GOLANGCI_LINT_VERSION)" ]; then \
		curl $(VERBOSE_CURL_FLAG)SfL https://golangci-lint.run/install.sh | sh -s -- -b $(TOOLS_DIR) $(GOLANGCI_LINT_VERSION); \
		echo "golangci-lint $(GOLANGCI_LINT_VERSION) installed"; \
	else \
		echo "golangci-lint $(GOLANGCI_LINT_VERSION) already installed"; \
	fi

.PHONY: install-goreleaser
install-goreleaser:
	$(Q)echo "Installing goreleaser $(GORELEASER_VERSION)..."
	$(Q)mkdir -p $(TOOLS_DIR)
	@if [ ! -f $(GORELEASER_BIN) ] || [ "$$($(GORELEASER_BIN) --version 2>/dev/null | grep -o 'v[0-9]\+\.[0-9]\+\.[0-9]\+')" != "$(GORELEASER_VERSION)" ]; then \
		curl $(VERBOSE_CURL_FLAG)SfL https://github.com/goreleaser/goreleaser/releases/download/$(GORELEASER_VERSION)/goreleaser_$(OS)_$(ARCH).tar.gz | tar -xz -C $(TOOLS_DIR) goreleaser; \
		chmod +x $(GORELEASER_BIN); \
		echo "goreleaser $(GORELEASER_VERSION) installed"; \
	else \
		echo "goreleaser $(GORELEASER_VERSION) already installed"; \
	fi

.PHONY: install-upx
install-upx:
	$(Q)echo "Installing upx $(UPX_VERSION)..."
	$(Q)mkdir -p $(TOOLS_DIR)
	@if [ ! -f $(UPX) ] || [ "$$($(UPX) --version 2>/dev/null | head -1 | grep -o 'v[0-9]\+\.[0-9]\+\.[0-9]\+')" != "$(UPX_VERSION)" ]; then \
		UPX_ARCH=$(ARCH); \
		if [ "$(ARCH)" = "x86_64" ]; then UPX_ARCH="amd64"; fi; \
		if [ "$(ARCH)" = "arm64" ]; then UPX_ARCH="arm64"; fi; \
		if [ "$(OS)" = "linux" ]; then \
			UPX_FILE="upx-$(UPX_VERSION:v%=%)-$${UPX_ARCH}_linux"; \
		elif [ "$(OS)" = "darwin" ]; then \
			UPX_FILE="upx-$(UPX_VERSION:v%=%)-amd64_macos"; \
		else \
			echo "Error: Unsupported OS $(OS) for UPX installation"; \
			exit 1; \
		fi; \
		curl $(VERBOSE_CURL_FLAG)SfL https://github.com/upx/upx/releases/download/$(UPX_VERSION)/$${UPX_FILE}.tar.xz | tar -xJ -C $(TOOLS_DIR) --strip-components=1 $${UPX_FILE}/upx; \
		chmod +x $(UPX); \
		echo "upx $(UPX_VERSION) installed"; \
	else \
		echo "upx $(UPX_VERSION) already installed"; \
	fi

.PHONY: generate-mock
generate-mock: install-mockery
	$(MOCKERY)

# The Serverless Application Repository renders the application READMEs
# (ReadmeUrl: SAR.md, and SAR-next-gen.md via next-gen.patch) as basic
# Markdown only: no tables and no GitHub alerts (> [!NOTE] and so on; a plain
# > blockquote with a bold label works). Fails if either README uses them (#406).
SAR_READMES = SAR.md SAR-next-gen.md
.PHONY: sar-readmes
sar-readmes:
	@ls $(SAR_READMES) >/dev/null
	@if grep -n -H -i -E '^[[:space:]]*\||\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]' $(SAR_READMES); then \
	  echo "error: SAR READMEs must be basic Markdown: replace table rows (lines starting with |) with lists, and GitHub alerts with '> **Note:** ...' blockquotes (#406)"; \
	  exit 1; \
	fi
	@echo "sar-readmes: ok"
.PHONY: test
test: generate-mock sar-readmes
	$(Q)go test $(VERBOSE_TEST_FLAG) `go list ./internal/... | grep -v ./internal/mocks` -coverprofile=coverage.out

# Unit tests of the cicd/tests Python helpers. One discover per directory:
# each imports its modules from its own directory. Needs PyYAML. make's
# wildcard has no **, so this looks up to three directories below cicd/tests;
# add a level here for a test deeper than that.
CICD_TEST_DIRS = $(sort $(dir $(wildcard cicd/tests/*/test_*.py cicd/tests/*/*/test_*.py cicd/tests/*/*/*/test_*.py)))
.PHONY: test-cicd
test-cicd:
	$(Q)set -e; for d in $(CICD_TEST_DIRS); do \
	  echo "test-cicd: $$d"; \
	  python3 -B -m unittest discover $(VERBOSE_TEST_FLAG) -s $$d -p 'test_*.py'; \
	done

.PHONY: test-verbose
test-verbose: generate-mock
	$(Q)go test -v `go list ./internal/... | grep -v ./internal/mocks` -coverprofile=coverage.out

.PHONY: test-coverage
test-coverage: test
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	go tool cover -func=coverage.out

.PHONY: go-build
go-build: install-goreleaser install-upx
	$(Q)$(GORELEASER_BIN) build --snapshot --clean --id ssosync $(VERBOSE_BUILD_FLAG) $(GOREL_ARGS)

.PHONY: go-build-lambda
go-build-lambda: install-goreleaser
	$(Q)$(GORELEASER_BIN) build --snapshot --clean --id ssosync $(VERBOSE_BUILD_FLAG) $(GOREL_ARGS)

# The two SAR applications (issue #399), each with a staging and a release
# variant, are template.yaml with patches from cicd/build/package/. Each patch
# is unique and atomic:
#   legacy.patch            removes the next-gen features only; never touches Name
#   next-gen.patch          removes the legacy features only; never touches Name
#   legacy-release.patch    renames ssosync to SSOSync; Name line only
#   legacy-staging.patch    renames ssosync to SSOSync-Staging; Name line only
#   next-gen-staging.patch  renames ssosync to ssosync-staging; Name line only
# There is no next-gen-release.patch: template.yaml's default Name, ssosync, is
# already the next-gen release name. Both legacy name patches apply to
# legacy.yaml (never one after the other). The four combinations, with the
# names cicd/build/package/buildspec.yml (which runs this target) packages
# them as:
#   legacy.patch + legacy-release.patch      => SSOSync          (release.yaml)
#   legacy.patch + legacy-staging.patch      => SSOSync-Staging  (staging.yaml)
#   next-gen.patch                           => ssosync          (ng-release.yaml)
#   next-gen.patch + next-gen-staging.patch  => ssosync-staging  (ng-staging.yaml)
# Every patch is applied with no fuzz (patch -N -F 0), so a patch whose context
# no longer matches exactly fails here (and so fails the build). Writes the
# variants to dist/sar/ (and lints the feature variants when cfn-lint is
# installed; it is skipped otherwise, as on CodeBuild); a failed patch prints
# its rejects and leaves its .rej there.
#
# The patches are not regenerated automatically. To change one (<app> is legacy
# or next-gen, <stage> is staging or release):
#   Feature patch <app>.patch, against template.yaml:
#     1. make sar-variants (if it fails on <app>.patch itself, build
#        dist/sar/<app>.yaml by hand from template.yaml instead)
#     2. edit dist/sar/<app>.yaml; leave its Name as ssosync
#     3. diff -u --label template.yaml --label <app>.yaml template.yaml dist/sar/<app>.yaml > cicd/build/package/<app>.patch
#   Name patch <app>-<stage>.patch, against the feature variant <app>.yaml:
#     1. make sar-variants (the feature variants are written first)
#     2. cp dist/sar/<app>.yaml dist/sar/<app>-<stage>.yaml, and set the Name
#        line of its AWS::ServerlessRepo::Application block
#     3. diff -u --label <app>.yaml --label <app>-<stage>.yaml dist/sar/<app>.yaml dist/sar/<app>-<stage>.yaml > cicd/build/package/<app>-<stage>.patch
#   Then rerun make sar-variants. A feature-patch change to the lines around
#   Name also needs that app's name patches regenerated.
#   A name patch must change only the Name line: a staging template and its
#   release template may differ in nothing else. sar-variants checks this
#   and fails otherwise.
.PHONY: sar-variants
sar-variants: sar-readmes
	@mkdir -p dist/sar && rm -f dist/sar/*.orig dist/sar/*.rej
	@set -e; p=cicd/build/package; \
	apply() { patch -N -F 0 dist/sar/$$1.yaml $$p/$$2.patch || { \
	  cat dist/sar/$$1.yaml.rej >&2 || true; \
	  echo "error: $$p/$$2.patch does not apply to dist/sar/$$1.yaml with no fuzz; regenerate it (see above sar-variants in the Makefile)" >&2; \
	  exit 1; }; }; \
	for v in legacy next-gen; do \
	  cp template.yaml dist/sar/$$v.yaml; apply $$v $$v; done; \
	for v in legacy-staging legacy-release next-gen-staging; do \
	  cp dist/sar/$${v%-*}.yaml dist/sar/$$v.yaml; apply $$v $$v; done; \
	cp dist/sar/next-gen.yaml dist/sar/next-gen-release.yaml
	@grep -H -E '^    (Name|ReadmeUrl):' dist/sar/*-staging.yaml dist/sar/*-release.yaml
	@# Staging and release must differ only in the SAR Name line: each has
	@# exactly one "    Name:" line, and diff -I ignores only that line.
	@set -e; for v in legacy next-gen; do \
	  for f in dist/sar/$$v-staging.yaml dist/sar/$$v-release.yaml; do \
	    [ "$$(grep -c '^    Name:' $$f)" = 1 ] || { echo "error: $$f must have exactly one '    Name:' line" >&2; exit 1; }; done; \
	  diff -I '^    Name:' dist/sar/$$v-staging.yaml dist/sar/$$v-release.yaml || { \
	    echo "error: dist/sar/$$v-staging.yaml and dist/sar/$$v-release.yaml must differ only in their Name line; a name patch may change only that line" >&2; \
	    exit 1; }; \
	done; echo "staging vs release, Name only: ok"
	@if command -v cfn-lint >/dev/null 2>&1; then cfn-lint -i W1030,W1031 -- dist/sar/legacy.yaml dist/sar/next-gen.yaml && echo "cfn-lint: ok"; fi

.PHONY: clean
clean:
	rm -f $(OUTPUT) $(PACKAGED_TEMPLATE) bootstrap coverage.out coverage.html
	rm -rf dist/ internal/mocks/*

.PHONY: clean-all
clean-all: clean
	rm -rf $(TOOLS_DIR)

build-SSOSyncFunction: go-build
	cp dist/ssosync_linux_arm64_v8.2/ssosync $(ARTIFACTS_DIR)/bootstrap

.PHONY: config
config:
	go mod download

.PHONY: vet
vet: install-golangci-lint generate-mock
	$(GOLANGCI_LINT) run

.PHONY: lint
lint: vet

.PHONY: fmt
fmt:
	$(Q)go fmt $(VERBOSE_FLAG) ./...
	$(Q)go mod tidy $(VERBOSE_FLAG)

main: main.go install-goreleaser
	$(Q)echo $(GORELEASER_BIN) build --clean $(VERBOSE_BUILD_FLAG) $(GOREL_ARGS)
	$(Q)$(GORELEASER_BIN) build --clean $(VERBOSE_BUILD_FLAG) $(GOREL_ARGS)

# compile the code to run in Lambda (local or real)
.PHONY: lambda
lambda: main

.PHONY: build
build: clean main

.PHONY: release
release: go-build
	$(Q)$(GORELEASER_BIN) release --clean $(VERBOSE_BUILD_FLAG) $(GOREL_ARGS)

.PHONY: dry-run
dry-run: 
	$(MAKE) GOREL_ARGS=--skip=publish release

.PHONY: api
api: build
	sam local start-api

.PHONY: publish
publish:
	sam publish -t packaged.yaml

.PHONY: package
package: build
	@if [ "$(ARCH)" = "arm64" ]; then \
		cp dist/ssosync_$(OS)_arm64_v8.2/ssosync ./bootstrap; \
	elif [ "$(ARCH)" = "x86_64" ]; then \
		cp dist/ssosync_$(OS)_amd64_v1/ssosync ./bootstrap; \
	else \
		echo "Error: Unsupported architecture $(ARCH)"; \
		exit 1; \
	fi
	sam package --s3-bucket $(S3_BUCKET) --output-template-file $(PACKAGED_TEMPLATE) --s3-prefix $(S3_PREFIX)

.PHONY: deploy
deploy: package
	sam deploy --stack-name $(STACK_NAME) --template-file $(PACKAGED_TEMPLATE) --capabilities CAPABILITY_IAM



.PHONY: dev
dev: go-build
	@echo "Running development build for $(OS)/$(ARCH)..."
	@if [ "$(ARCH)" = "arm64" ]; then \
		BINARY_PATH="dist/ssosync_$(OS)_arm64_v8.2/ssosync"; \
	elif [ "$(ARCH)" = "x86_64" ]; then \
		BINARY_PATH="dist/ssosync_$(OS)_amd64_v1/ssosync"; \
	else \
		BINARY_PATH=$$(find dist/ -name "ssosync_$(OS)_*" -type f | head -1); \
	fi; \
	if [ ! -f "$$BINARY_PATH" ]; then \
		echo "Error: Binary not found at $$BINARY_PATH. Available binaries:"; \
		find dist/ -name "ssosync*" -type f || echo "No binaries found in dist/"; \
		exit 1; \
	fi; \
	echo "Using binary: $$BINARY_PATH"; \
	$$BINARY_PATH -g "name:AWS*" \
		-t $$(jq '.["ssosync/aws-sso/scimEndpointAccessToken"]' ./cicd/cloudformation/cdk/cdk.context.json -r) \
		-r $$(jq '.["ssosync/aws-sso/region"]' ./cicd/cloudformation/cdk/cdk.context.json -r) \
		-e $$(jq '.["ssosync/aws-sso/scimEndpointUrl"]' ./cicd/cloudformation/cdk/cdk.context.json -r) \
		-u $$(jq '.["ssosync/secrets/googleAdminEmail"]' ./cicd/cloudformation/cdk/cdk.context.json -r) \
		-i $$(jq '.["ssosync/aws-sso/identityStoreId"]' ./cicd/cloudformation/cdk/cdk.context.json -r) \
		-c ./cicd/cloudformation/cdk/google-service-account.json \
		--log-level debug
		
.PHONY: check-tools
check-tools:
	@echo "Checking installed tools..."
	@if [ -f $(MOCKERY) ]; then echo "✓ mockery: $$($(MOCKERY) version)"; else echo "✗ mockery: not installed"; fi
	@if [ -f $(GOLANGCI_LINT) ]; then echo "✓ golangci-lint: $$($(GOLANGCI_LINT) --version)"; else echo "✗ golangci-lint: not installed"; fi
	@if [ -f $(GORELEASER_BIN) ]; then echo "✓ goreleaser: $$($(GORELEASER_BIN) --version)"; else echo "✗ goreleaser: not installed"; fi
	@if [ -f $(UPX) ]; then echo "✓ upx: $$($(UPX) --version 2>/dev/null | head -1)"; else echo "✗ upx: not installed"; fi
	@echo "Go version: $$(go version)"

.PHONY: setup
setup: install-deps config
	@echo "Development environment setup complete"

.PHONY: ci
ci: vet test sar-variants test-cicd
	@echo "CI pipeline completed successfully"

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  setup           - Install all dependencies and setup development environment"
	@echo "  install-deps    - Install all development dependencies (mockery, golangci-lint, goreleaser, upx)"
	@echo "  check-tools     - Check status of installed tools"
	@echo "  fmt             - Format code and tidy modules"
	@echo "  generate-mock   - Generate mocks using mockery"
	@echo "  test            - Run tests with coverage"
	@echo "  sar-readmes     - Check SAR.md and SAR-next-gen.md use only basic Markdown (run by test)"
	@echo "  sar-variants    - Build and check the four SAR templates in dist/sar/ (run by ci)"
	@echo "  test-cicd       - Run the cicd/tests Python helper unit tests (run by ci)"
	@echo "  test-verbose    - Run tests with verbose output"
	@echo "  test-coverage   - Generate HTML coverage report"
	@echo "  vet/lint        - Run linters"
	@echo "  go-build        - Build application using goreleaser"
	@echo "  build           - Clean and build application"
	@echo "  release         - Create release using goreleaser"
	@echo "  dry-run         - Test release without publishing"
	@echo "  clean           - Clean build artifacts"
	@echo "  clean-all       - Clean everything including development tools (.bin/)"
	@echo "  ci              - Run CI pipeline (vet, test, sar-variants, test-cicd)"
	@echo "  dev             - Run development build (auto-detects $(OS)/$(ARCH))"
	@echo "  dev-help        - Show help for development build"
	@echo "  help            - Show this help message"
	@echo ""
	@echo "Verbose mode:"
	@echo "  Use VERBOSE=1 with any target for verbose output (e.g., 'make test VERBOSE=1')"

.PHONY: dev-help
dev-help: go-build
	@echo "Running development build help for $(OS)/$(ARCH)..."
	@if [ "$(ARCH)" = "arm64" ]; then \
		BINARY_PATH="dist/ssosync_$(OS)_arm64_v8.2/ssosync"; \
	elif [ "$(ARCH)" = "x86_64" ]; then \
		BINARY_PATH="dist/ssosync_$(OS)_amd64_v1/ssosync"; \
	else \
		BINARY_PATH=$$(find dist/ -name "ssosync_$(OS)_*" -type f | head -1); \
	fi; \
	if [ ! -f "$$BINARY_PATH" ]; then \
		echo "Error: Binary not found at $$BINARY_PATH"; \
		exit 1; \
	fi; \
	echo "Using binary: $$BINARY_PATH"; \
	$$BINARY_PATH --help
