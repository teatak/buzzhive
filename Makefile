IMAGE ?= teatak/buzzhive
TAG ?= latest
PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: dev admin-build admin-dev docker-build docker-push docker-publish version-patch version-minor version-major

dev: admin-build
	@test -f config.yaml || cp config.example.yaml config.yaml
	docker compose -f docker-compose.dev.yml up -d postgres redis
	go run ./cmd/buzzhive -config config.yaml

admin-build:
	cd admin && pnpm install --frozen-lockfile --config.confirm-modules-purge=false && pnpm build

admin-dev:
	cd admin && pnpm install --config.confirm-modules-purge=false && pnpm dev

docker-build:
	docker build -t $(IMAGE):$(TAG) .

docker-push:
	docker push $(IMAGE):$(TAG)

docker-publish:
	docker buildx build --platform $(PLATFORMS) -t $(IMAGE):$(TAG) -t $(IMAGE):$$(cat VERSION) --push .

version-patch:
	@awk -F. '{ printf "%d.%d.%d\n", $$1, $$2, $$3 + 1 }' VERSION > VERSION.tmp
	@mv VERSION.tmp VERSION
	@cat VERSION

version-minor:
	@awk -F. '{ printf "%d.%d.0\n", $$1, $$2 + 1 }' VERSION > VERSION.tmp
	@mv VERSION.tmp VERSION
	@cat VERSION

version-major:
	@awk -F. '{ printf "%d.0.0\n", $$1 + 1 }' VERSION > VERSION.tmp
	@mv VERSION.tmp VERSION
	@cat VERSION
