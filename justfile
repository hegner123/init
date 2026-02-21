default:
    @just --list

build:
    go build -o init .

test:
    go test ./...

install: build
    cp init /usr/local/bin/init

clean:
    rm -f init
