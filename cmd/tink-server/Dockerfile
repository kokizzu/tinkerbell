FROM alpine:3.20.3

ARG TARGETOS
ARG TARGETARCH

EXPOSE 42113 42114

RUN apk add --no-cache --update --upgrade ca-certificates=20241121-r1

COPY bin/tink-server-${TARGETOS}-${TARGETARCH} /usr/bin/tink-server

ENTRYPOINT ["/usr/bin/tink-server"]
