# The image has the environment setup for running builds and tests
# on Google Cloud Build.
# use golang 1.26.5
FROM golang:1.26.5-bookworm
RUN apt-get update \
    && apt-get install -y --no-install-recommends python3-dev
