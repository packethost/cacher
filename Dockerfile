FROM alpine:3.7

CMD /cacher
RUN apk add --no-cache --update --upgrade ca-certificates
ADD cacher /cacher
