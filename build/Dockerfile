FROM alpine:latest

LABEL maintainer Fred <fred@gcreativeprojects.tech>

ARG TARGETPLATFORM

COPY $TARGETPLATFORM/imap /usr/bin/imap

VOLUME /imap
WORKDIR /imap

ENTRYPOINT ["imap"]
CMD ["--help"]
