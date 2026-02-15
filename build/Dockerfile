FROM alpine:latest

LABEL maintainer Fred <fred@gcreativeprojects.tech>

ARG ARCH=amd64
ARG TARGETPLATFORM

COPY $TARGETPLATFORM/imap /usr/bin/imap

VOLUME /imap
WORKDIR /imap

ENTRYPOINT ["imap"]
CMD ["--help"]
