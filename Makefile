all: build-all

build-all: build-linux

build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o whatsappbot whatsappbot.go nicks.go

receive:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o receive receive.go
