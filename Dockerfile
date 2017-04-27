FROM alpine:3.5

ADD ./dist/cmd /bin/cmd

CMD ["cmd"]
