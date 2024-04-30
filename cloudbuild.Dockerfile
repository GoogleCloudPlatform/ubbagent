# The image has the environment setup for running builds and tests
# on Google Cloud Build.
FROM gcr.io/cloud-builders/go:debian-1.21
RUN apt-get update \
    && apt-get install -y --no-install-recommends python3-dev
