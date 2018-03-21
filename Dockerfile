# Copyright 2018 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.FROM golang:1.9.2-alpine3.7 AS build

FROM golang:1.9.2-alpine3.7 AS build

COPY . /ubbagent-src/
WORKDIR /ubbagent-src/

RUN apk add --no-cache make git
RUN rm -rf /ubbagent-src/.git
RUN make clean setup deps build

FROM alpine:3.7
RUN apk add --update libintl ca-certificates && \
    apk add --virtual build_deps gettext && \
    cp /usr/bin/envsubst /usr/local/bin/envsubst && \
    apk del build_deps && \
    rm -rf /var/cache/apk/*

COPY --from=build /ubbagent-src/bin/ubbagent /usr/local/bin/ubbagent
COPY docker/ubbagent-start /usr/local/bin/ubbagent-start
CMD ["/usr/local/bin/ubbagent-start"]
