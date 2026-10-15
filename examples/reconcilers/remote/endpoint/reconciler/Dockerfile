FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    socat \
    jq \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /reconciler
COPY reconciler.sh .
RUN chmod +x reconciler.sh

EXPOSE 8025

ENTRYPOINT ["./reconciler.sh"]
