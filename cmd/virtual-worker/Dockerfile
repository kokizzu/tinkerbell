FROM alpine:3.20.3

ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache --update --upgrade ca-certificates=20241121-r1

COPY bin/virtual-worker-${TARGETOS}-${TARGETARCH} /usr/bin/virtual-worker

ENTRYPOINT [ "/usr/bin/virtual-worker" ]
