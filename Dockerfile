FROM alpine:3.11

ENTRYPOINT [ "/entrypoint.sh" ]
CMD [ "/cacher" ]
EXPOSE 42111
EXPOSE 42112

RUN apk add --no-cache --update --upgrade ca-certificates
RUN apk add --no-cache --update --upgrade --repository=http://dl-cdn.alpinelinux.org/alpine/edge/testing cfssl
COPY entrypoint.sh /entrypoint.sh
COPY tls /tls
COPY cacher-linux-x86_64 /cacher
