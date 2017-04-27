FROM golang:1.8

ADD . /src/

WORKDIR /src/

RUN go install ./cmd/

CMD ["cmd"]
