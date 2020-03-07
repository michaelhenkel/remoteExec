FROM alpine:3.9
COPY server/server /server
CMD ["/server"]
