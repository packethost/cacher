FROM alpine:3.11

ENTRYPOINT [ "/entrypoint.sh" ]
CMD [ "/cacher" ]
ENV GRPC_PORT=42111
EXPOSE $GRPC_PORT
ENV HTTP_PORT=42112
EXPOSE $HTTP_PORT

RUN apk add --no-cache --update --upgrade ca-certificates
RUN apk add --no-cache --update --upgrade --repository=http://dl-cdn.alpinelinux.org/alpine/edge/testing cfssl
COPY entrypoint.sh /entrypoint.sh
COPY tls /tls
COPY cacher-linux-x86_64 /cacher
