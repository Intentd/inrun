FROM python:3.13-slim

WORKDIR /reconciler
COPY main.py .

USER 65532:65532
EXPOSE 8025

ENTRYPOINT ["python3", "main.py"]
