# docker buildx create --use
docker buildx build --load --platform=linux/amd64,linux/arm64 --tag "devforth/onlogs:latest" --tag "devforth/onlogs:1.1.4" .
