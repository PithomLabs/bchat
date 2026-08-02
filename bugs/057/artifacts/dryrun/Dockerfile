FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY probe /probe
RUN chmod +x /probe
EXPOSE 5231
CMD ["/probe"]
