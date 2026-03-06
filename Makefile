#
# Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
#
# SPDX-License-Identifier: BSD-2-Clause
#

.PHONY: run-demo
run-demo:
	go run ./cmd/attestam -insecure-demo-mode

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
