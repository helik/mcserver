FROM golang:alpine as builder

RUN mkdir -p /go/src/github.com/helik/mcserver

RUN apk --update add git

RUN go get github.com/kardianos/govendor

WORKDIR /go/src/github.com/helik/mcserver

COPY vendor/vendor.json vendor/vendor.json
RUN /go/bin/govendor sync
RUN /go/bin/govendor install +vendor

COPY controller/ controller/
COPY main.go main.go
RUN go build -o /mcserver


FROM openjdk:alpine

COPY --from=builder /mcserver /mcserver

COPY minecraft_server.jar minecraft_server.jar
COPY server.properties server.properties
COPY eula.txt eula.txt

EXPOSE 25565

CMD /mcserver -endpoint=$ENDPOINT -accesskey=$ACCESSKEY -secretkey=$SECRETKEY -usessl=$USESSL -bucket=$BUCKET -name=$NAME