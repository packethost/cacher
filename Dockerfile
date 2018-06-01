FROM alpine:3.7

ENTRYPOINT [ "/entrypoint.sh" ]
CMD [ "/cacher" ]

RUN apk add --no-cache --update --upgrade ca-certificates
RUN apk add --no-cache --update --upgrade --repository=http://dl-cdn.alpinelinux.org/alpine/edge/testing cfssl
ADD entrypoint.sh /entrypoint.sh
ADD tls /tls
ADD cacher /cacher
