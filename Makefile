.PHONY: lint format test check

lint:
	ruff check backend/

format:
	ruff format backend/

test:
	pytest -v --cov=backend --cov-report=term-missing

check: lint test
