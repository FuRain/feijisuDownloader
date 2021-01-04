PROJECT_NAME := "feijisu"
TARGET=feijisu
LDFLAGS="-s -w"

all: build

build: clean
	@go build -o ${TARGET} -v -ldflags=${LDFLAGS} .

windows:
	@GOOS=windows GOARCH=386 go build -o ${TARGET}.exe .

clean:
	@rm -rf ${TARGET}
