PROJECT_NAME := "feijisu"
TARGET=feijisu
LDFLAGS="-s -w"

all: build windows

build: clean
	@go build -o ${TARGET} -v -ldflags=${LDFLAGS} .

windows: clean
	@GOOS=windows GOARCH=386 go build -o ${TARGET}.exe .

clean:
	@rm -rf ${TARGET}
	@rm -rf ${TARGET}.exe
