# docker buildx create --use
docker buildx build --platform=linux/amd64,linux/arm64 --tag "devforth/onlogs:latest" --tag "devforth/onlogs:1.1.9" --push .
# docker run -v /var/run/docker.sock:/var/run/docker.sock --rm -it $(docker build -q -f Dockerfile .)
# docker build . -t devforth/onlogs && docker push devforth/onlogs
