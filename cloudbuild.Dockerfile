FROM gcr.io/cloud-builders/go:debian
RUN apt-get update \
    && apt-get install -y --no-install-recommends python2-dev python3-dev
