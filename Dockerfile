FROM --platform=${BUILDPLATFORM} golang:1.25 as build-env

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

RUN apt-get update && apt-get install -yq --no-install-recommends git

# Copy source + vendor
COPY . /go/src/github.com/vincejv/gpon-parser
WORKDIR /go/src/github.com/vincejv/gpon-parser

# Compile go binaries
ENV GOPATH=/go
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GO111MODULE=on go build -v -a -ldflags "-s -w" -o /go/bin/gpon-parser .

# Build final image from alpine
FROM --platform=${TARGETPLATFORM} alpine:latest
RUN apk --update --no-cache add curl && rm -rf /var/cache/apk/*
COPY --from=build-env /go/bin/gpon-parser /usr/bin/gpon-parser

# Create a group and user
RUN addgroup -S gpon-parser && adduser -S gpon-parser -G gpon-parser
USER gpon-parser

EXPOSE 8092
ENTRYPOINT ["gpon-parser"]
HEALTHCHECK --interval=30s --timeout=10s --retries=5 \
  CMD curl -f http://localhost:8092/health || exit 1