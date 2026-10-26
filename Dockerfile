FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETARCH
COPY inrun-${TARGETARCH} /usr/local/bin/inrun

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/inrun"]
