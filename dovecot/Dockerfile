FROM alpine:3.13

RUN apk add dovecot
RUN addgroup -g 1000 vmail && \
    adduser vmail -u 1000 -G vmail -h /srv/vmail -D && \
    mkdir -p /srv/mail && \
    chown vmail:vmail /srv/mail && \
    rm -f /etc/dovecot/conf.d/10-auth.conf

COPY local.conf /etc/dovecot/local.conf

CMD ["/usr/sbin/dovecot", "-F"]
