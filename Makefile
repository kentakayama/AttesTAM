.PHONY: run-demo
run-demo:
	go run ./cmd/tam-over-http -insecure-demo-mode

.PHONY: test
test:
	go test ./...

.PHONY: test-integrated
test-integrated:
	go test -tags=integration ./...

.PHONY: clean
clean:
	@echo "[WARNING] Are you sure to clear the TAM's Status?"
	@$(RM) -i tam_state.db*
