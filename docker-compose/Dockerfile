ARG builder_image=golang:alpine
ARG base_image=docker

FROM ${builder_image} AS builder
WORKDIR /src
COPY . .
RUN go build -o ../prototype .


##########################


FROM ${base_image}

ARG docker_compose_version

RUN apk add --no-cache py3-pip python3 util-linux iptables device-mapper
RUN apk add --no-cache --virtual \
  build-dependencies \
  cargo \
  gcc \
  libc-dev \
  libffi-dev \
  make \
  openssl-dev \
  python3-dev \
  rust \
  && pip3 install "docker-compose${COMPOSE_VERSION:+==}${COMPOSE_VERSION}" \
  && apk del build-dependencies \
  && rm -rf /root/.cache \
  && rm -rf /root/.cargo

COPY --from=builder /prototype /bin/prototype
COPY start-docker.sh /bin/start-docker
COPY stop-docker.sh /bin/stop-docker

ENTRYPOINT ["prototype"]
