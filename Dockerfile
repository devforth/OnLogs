FROM node:16-alpine AS frontbuilder

WORKDIR /code/
ADD frontend/package-lock.json .
ADD frontend/package.json .
RUN npm install -g npm@latest && npm ci

ADD frontend/. .

RUN npm run build

COPY . /code/

FROM alpine
RUN apk add bash curl
# tmp

COPY --from=frontbuilder /code/dist/ /backend/dist/

FROM golang:alpine AS backendbuilder

ADD backend/. /backend/
WORKDIR /backend/

RUN go mod download  \
  && go build -o main .

FROM alpine
RUN apk add bash curl
# tmp

EXPOSE 2874

COPY --from=frontbuilder /code/dist/ /dist/
COPY --from=backendbuilder /backend/main /backend/main
CMD ["/backend/main"]

# docker run -v /var/run/docker.sock:/var/run/docker.sock --rm -it $(docker build -f Dockerfile .)
# docker build . -t devforth/onlogs && docker push devforth/onlogs
# docker run --restart always devforth/onlogs -e PASSWORD=notqwertyplesnever -e JWT_TOKEN=amogus12 -e PORT=2874 -e CLIENT=true -e HOST=onlogs.coposter.me -e ONLOGS_TOKEN=NDBYGFI7OLfgvuDgvoD98  -v /var/run/docker.sock:/var/run/docker.sock -v /etc/hostname:/etc/hostname -v onlogs-volume:/backend/leveldb -"traefik.enable=true"