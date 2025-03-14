PRJ=tsmetrics
BINS=$(PRJ).linux.amd64 $(PRJ).darwin.arm64 $(PRJ).windows.amd64

.PHONY: checks vuln
checks: lint vuln test

vuln:
	govulncheck ./...

test:
	go test -v *.go

test/watch:
	@ls *.go | entr -c -s 'go test -failfast -v ./*.go && notify "💚" || notify "🛑"'

coverage/html:
	go test -v -cover -coverprofile=c.out
	go tool cover -html=c.out

.PHONY:run
run:
	@bash -c 'set -a; source <(cat .env | \
		sed "s/#.*//g" | xargs); \
		set +a && go run . --wait-secs=240 --tsnet-verbose --addr=:9100 --resolve-names'

.PHONY: run/local
run/local:
	@bash -c 'set -a; source <(cat .env | \
		sed "s/#.*//g" | xargs); \
		set +a && go run . --addr=:9100 --regular-server --resolve-names'

build: $(BINS)

$(PRJ).linux.amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $@ .

$(PRJ).darwin.arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $@ .

$(PRJ).windows.amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $@ .

clean:
	rm -f c.out $(BINS) cert-cacher $(BINS)

.PHONY: module
module:
	rm -f *.mod
	go mod init github.com/drio/$(PRJ)
	go mod tidy	

.PHONY: lint
lint:
	golangci-lint run

.PHONY: deps
deps:
	brew install golangci-lint
	go install golang.org/x/vuln/cmd/govulncheck@latest

update:
	go get -u ./...
	go mod tidy

rsync:
	rsync -avz -e ssh --progress --exclude=tsmetrics/tsmetrics ../tsmetrics prom-graf:services/
