FROM alpine:3.20
RUN apk add --no-cache postfix cyrus-sasl ca-certificates bash
RUN adduser -S -D -H -s /sbin/nologin postfix
COPY main.cf /etc/postfix/main.cf
EXPOSE 25
CMD ["/usr/sbin/postfix", "start-fg"]
