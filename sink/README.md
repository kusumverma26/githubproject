# HTTP Sink

Takes any POST and GET requests and logs them

## Setup sink

```bash
docker build -t my-sink .
docker run -d -p 9009:9009 --name=sink my-sink
```

## Helper Commands

Display logs

```bash
docker logs -f sink
```
