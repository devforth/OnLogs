FROM node:16-alpine AS frontbuilder

WORKDIR /code/
ADD frontend/package-lock.json .
ADD frontend/package.json .
RUN npm install -g npm:16 && npm ci

ADD frontend/. .

RUN npm run build

COPY . /code/

FROM alpine

COPY --from=frontbuilder /code/dist/ /backend/dist/

FROM golang:alpine AS backendbuilder

ADD backend/. /backend/
WORKDIR /backend/

RUN go mod download  \
  && go build -o main .

FROM alpine

COPY --from=frontbuilder /code/dist/ /dist/
COPY --from=backendbuilder /backend/main /backend/main
CMD ["/backend/main"]

# docker run -v /var/run/docker.sock:/var/run/docker.sock --rm -it $(docker build -f Dockerfile .)
# docker build . -t devforth/onlogs && docker push devforth/onlogs
