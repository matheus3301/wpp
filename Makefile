.PHONY: references

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
