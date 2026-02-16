.PHONY: references fmt tidy vet test lint

references:
	@mkdir -p .references
	@set -e; \
	repos='Baileys=https://github.com/WhiskeySockets/Baileys.git evolution-api=https://github.com/EvolutionAPI/evolution-api.git k9s=https://github.com/derailed/k9s.git wacli=https://github.com/steipete/wacli.git whatsapp-cli=https://github.com/vicentereig/whatsapp-cli.git'; \
	for entry in $$repos; do \
		name=$${entry%%=*}; \
		url=$${entry#*=}; \
		dest=".references/$$name"; \
		if [ -d "$$dest/.git" ]; then \
			echo "skip $$name (already exists)"; \
		else \
			echo "clone $$name from $$url"; \
			git clone "$$url" "$$dest"; \
		fi; \
	done

fmt:
	@files="$$(find . \
		-type d \( -name .git -o -name .references -o -name bin \) -prune -o \
		-type f -name '*.go' -print)"; \
	if [ -z "$$files" ]; then \
		echo "skip: no Go files found"; \
		exit 0; \
	fi; \
	echo "$$files" | xargs gofmt -w; \
	echo "ok: gofmt applied"

tidy:
	@echo "run: go mod tidy"
	@go mod tidy

vet:
	@pkgs="$$(go list ./... 2>/dev/null || true)"; \
	if [ -z "$$pkgs" ]; then \
		echo "skip: no Go packages found"; \
		exit 0; \
	fi; \
	echo "run: go vet $$pkgs"; \
	go vet $$pkgs

test:
	@pkgs="$$(go list ./... 2>/dev/null || true)"; \
	if [ -z "$$pkgs" ]; then \
		echo "skip: no Go packages found"; \
		exit 0; \
	fi; \
	echo "run: go test $$pkgs"; \
	go test $$pkgs

lint: fmt tidy vet
	@pkgs="$$(go list ./... 2>/dev/null || true)"; \
	if [ -z "$$pkgs" ]; then \
		echo "skip: no Go packages found"; \
		exit 0; \
	fi; \
	echo "run: golangci-lint run --fix"; \
	golangci-lint run --fix
