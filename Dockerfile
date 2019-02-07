FROM alpine:3.7

ENTRYPOINT [ "/entrypoint.sh" ]
CMD [ "/cacher" ]
EXPOSE 42111
EXPOSE 42112

RUN apk add --no-cache --update --upgrade ca-certificates
RUN apk add --no-cache --update --upgrade --repository=http://dl-cdn.alpinelinux.org/alpine/edge/testing cfssl
COPY entrypoint.sh /entrypoint.sh
COPY tls /tls
COPY db/migrate /migrate
COPY db/docker-entrypoint-initdb.d/cacher-init.sql /init.sql
COPY cacher-linux-x86_64 /cacher
