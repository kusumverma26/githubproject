# Instructions

## Setup prometheus

```bash
docker build -t my-prometheus .
docker run -p 9090:9090 --name=prom my-prometheus
```
