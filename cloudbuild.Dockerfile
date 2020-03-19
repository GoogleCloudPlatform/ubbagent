# The image has the environment setup for running builds and tests
# on Google Cloud Build.
FROM gcr.io/cloud-builders/go:debian
# Install bazel.
RUN curl https://bazel.build/bazel-release.pub.gpg | apt-key add - \
    && echo "deb [arch=amd64] https://storage.googleapis.com/bazel-apt stable jdk1.8" | tee /etc/apt/sources.list.d/bazel.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends python2-dev python3-dev bazel patch
