FROM alpine:3.9
COPY server/server_linux /server
COPY ctr /ctr
CMD ["/server"]
