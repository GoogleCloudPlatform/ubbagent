# The image has the environment setup for running builds and tests
# on Google Cloud Build.
# use golang 1.26.6
FROM golang:1.26.6-bookworm
RUN apt-get update \
    && apt-get install -y --no-install-recommends python3-dev
