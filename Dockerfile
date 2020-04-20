# Compile stage
FROM registry.faza.io/golang:1.13.1 AS builder
RUN mkdir /go/src/apps
RUN echo "nobody:x:65534:65534:Nobody:/:" > /etc_passwd
ADD src /go/src/apps
WORKDIR /go/src/apps
RUN make build-docker

# Final stage
FROM registry.faza.io/golang:1.13.1
COPY --from=builder /etc_passwd /etc/passwd
COPY --from=builder /go/bin/app /app/finance

#USER appuser
EXPOSE $PORT
CMD ["/app/finance"]