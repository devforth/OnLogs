FROM node:24-alpine AS frontbuilder

WORKDIR /code/
ADD frontend/package-lock.json .
ADD frontend/package.json .
RUN npm ci

ADD frontend/. .

RUN npm run build

COPY . /code/

FROM golang:1.26-alpine AS backendbuilder

ARG VERSION=dev

ADD backend/. /backend/
WORKDIR /backend/

RUN go mod download  \
  && go build -ldflags "-X github.com/devforth/OnLogs/app/metrics.Version=${VERSION}" -o onlogs .

FROM alpine

COPY --from=frontbuilder /code/dist/ /dist/
COPY --from=backendbuilder /backend/onlogs /backend/onlogs
CMD ["/backend/onlogs"]
