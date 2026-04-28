FROM scratch
ARG BINARY
COPY bin/${BINARY} /app
ENTRYPOINT ["/app"]
