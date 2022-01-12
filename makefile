buildDevDocker: go.mod go.sum main.go 
	docker build -t byteplow/node-ddns-controller:$(shell git rev-parse --short HEAD) .

buildAndPublishDocker: go.mod go.sum main.go 
	docker buildx build --platform=linux/amd64,linux/arm64 --push -t byteplow/node-ddns-controller:$(shell git rev-parse --short HEAD) .