all: build-all

build-all: build-linux build-windows

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o whatsappbot whatsappbot.go nicks.go infile.go config.go

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o whatsappbot.exe whatsappbot.go nicks.go infile.go config.go

