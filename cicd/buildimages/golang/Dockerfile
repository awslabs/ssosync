FROM standard:5.0 AS build
#Install go.lang
RUN wget -q https://storage.googleapis.com/golang/go${ImageVersion}.linux-amd64.tar.gz \
 && tar -C / -xzf go${ImageVersion}.linux-amd64.tar.gz \
 && export PATH="/go/bin:$PATH" && export GOPATH="/go" && export PATH="$GOPATH/bin:$PATH" \
 && rm go${ImageVersion}.linux-amd64.tar.gz

# Install golint
RUN go install golang.org/x/lint/golint@latest

# Install staticcheck
RUN go install honnef.co/go/tools/cmd/staticcheck@latest

# Install Testify to use common assertions and mocks in tests
RUN go get -u github.com/stretchr/testify

# Install goreleaser
RUN go install github.com/goreleaser/goreleaser@latest
