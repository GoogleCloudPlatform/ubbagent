# The image has the environment setup for running builds and tests
# on Google Cloud Build.
FROM gcr.io/cloud-builders/go:debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends python2-dev python3-dev
