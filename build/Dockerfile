FROM alpine:latest

ARG TARGETPLATFORM

COPY $TARGETPLATFORM/imap /usr/bin/imap

VOLUME /imap
WORKDIR /imap

ENTRYPOINT ["imap"]
CMD ["--help"]
