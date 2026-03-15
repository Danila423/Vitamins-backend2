.PHONY: seed-catalog

seed-catalog:
	@test -n "$(DATABASE_URL)" || (echo "DATABASE_URL is required" >&2; exit 1)
	psql "$(DATABASE_URL)" -f internal/db/seed_vitamin_catalog.sql
