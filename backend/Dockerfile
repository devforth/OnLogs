FROM golang:alpine as builder
ADD . /code/
WORKDIR /code/
RUN go mod download  \
  && go build -o main .

FROM alpine
RUN apk add bash curl  
# tmp

EXPOSE 2874
COPY --from=builder /code/main /code/main
CMD ["/code/main"]
# docker run -v /var/run/docker.sock:/var/run/docker.sock --rm -it $(docker build -f Dockerfile .)
