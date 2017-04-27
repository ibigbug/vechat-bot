FROM golang:1.8


ENV APP_DIR $GOPATH/src/github.com/ibigbug/vechat-bot/

ADD . $APP_DIR

WORKDIR $APP_DIR

RUN go get -d ./cmd/

RUN go install ./cmd/

CMD ["cmd"]
