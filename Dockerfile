# The dedicated server of Apogée, for Dokku or any Docker host. The sky is
# saved in /data, to mount as a volume. Players join on TCP port 47110.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The version is VERSION if given, else the commit Dokku deploys (GIT_REV,
# passed with `dokku docker-options:add APP build "--build-arg GIT_REV"`).
ARG VERSION
ARG GIT_REV
RUN v="${VERSION:-$(echo "${GIT_REV:-docker}" | cut -c1-7)}" && \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$v" \
    -o /out/apogee-server ./cmd/apogee-server

FROM alpine:3.22
COPY --from=build /out/apogee-server /usr/bin/apogee-server
# The uid Dokku gives its storage directories.
USER 32767:32767
VOLUME /data
EXPOSE 47110
# APOGEE_NAME is the name of the sky; APOGEE_ARGS takes the other options
# of apogee-server (-rivals, -seed…). mDNS stays off: it cannot cross the
# internet, players join by address.
ENV APOGEE_NAME="Apogée" APOGEE_ARGS=""
# exec: the server receives SIGTERM itself, and saves the sky before leaving.
CMD ["sh", "-c", "exec apogee-server -data /data/sky.json -mdns=false -name \"$APOGEE_NAME\" $APOGEE_ARGS"]
