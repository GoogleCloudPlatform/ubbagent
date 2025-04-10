# The image has the environment setup for running builds and tests
# on Google Cloud Build.
# use golang 1.23
FROM golang:1.23.0-alpine3.20
RUN apk add --no-cache bash make git python3-dev
