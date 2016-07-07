.DEFAULT: all
.PHONY: all clean

all: docker/.fluxy.done

clean:
	go clean
	rm fluxd
	rm -f docker/.*.done

docker/.fluxy.done: docker/Dockerfile.fluxy fluxd
	docker build -t weaveworks/fluxy -f docker/Dockerfile.fluxy .
	touch $@

fluxd:
	CGO=0 GOOS=linux GOARCH=amd64 go build -o $@ cmd/fluxd/main.go
