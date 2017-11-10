FROM alpine

RUN apk add --no-cache --update --upgrade ca-certificates
ADD cacher /cacher
CMD /cacher
