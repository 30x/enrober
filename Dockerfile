FROM alpine:3.1

#Install ssl certs so we can connect to ssl services
RUN apk update
RUN apk add ca-certificates
RUN update-ca-certificates

COPY enrober /

CMD ["/enrober"]
