FROM golang:1.12.0
COPY . /go/src/github.com/alwinius/bow
WORKDIR /go/src/github.com/alwinius/bow
RUN make install

FROM node:9.11.1-alpine
WORKDIR /app
COPY ui /app
RUN yarn
RUN yarn run lint --no-fix
RUN yarn run build

FROM alpine:latest
RUN apk --no-cache add ca-certificates openssh

VOLUME /data
ENV XDG_DATA_HOME /data

COPY --from=0 /go/bin/bow /bin/bow
COPY --from=1 /app/dist /www
ENTRYPOINT ["/bin/bow"]
EXPOSE 9300