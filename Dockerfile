FROM golang:1.19-alpine AS builder

LABEL maintainer="acornsoft"

# Move to working directory (/build).
WORKDIR /build

# Copy and download dependency using go mod.
COPY go.mod go.sum ./
RUN go mod download

# Copy the code into the container.
COPY . .

# Set necessary environment variables needed for our image and build the API server.
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-s -w" -o kore-on .

FROM ubuntu:20.04

ENV TZ=Asia/Seoul
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

RUN apt-get update
RUN apt-get install -y curl vim python3 python3-pip openssh-server
RUN pip3 install --upgrade pip
RUN pip3 install --upgrade virtualenv
RUN python3 -m pip install ansible-core==2.12.3
RUN python3 -m pip install netaddr
RUN python3 -m pip install cryptography
RUN ansible-galaxy collection install --ignore-certs community.docker
RUN ansible-galaxy collection install --ignore-certs kubernetes.core
RUN ansible-galaxy collection install --ignore-certs community.general
RUN ansible-galaxy collection install --ignore-certs community.crypto
RUN ansible-galaxy collection install --ignore-certs ansible.posix
RUN ansible-galaxy collection install --ignore-certs ansible.utils
RUN curl -O https://get.helm.sh/helm-v3.10.3-linux-amd64.tar.gz
RUN tar -zxvf helm-v3.10.3-linux-amd64.tar.gz
RUN mv linux-amd64/helm /usr/bin/helm 

# Copy binary and config files from /build to root folder of scratch container.
COPY --from=builder ["/build/kore-on", "/"]
COPY --from=builder ["/build/conf", "/conf"]
COPY internal /internal
COPY tools /tools
COPY ansible.cfg /ansible.cfg

# Command to run when starting the container.
