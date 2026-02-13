.PHONY: run
run:
	go run ./cmd/tam-over-http

.PHONY: test
test:
	go test ./...

.PHONY: test-integrated
test-integrated:
	go test -tags=integration ./...

.PHONY: clean
clean:
	rm -f app.wasm manifest.app.wasm.0.suit
