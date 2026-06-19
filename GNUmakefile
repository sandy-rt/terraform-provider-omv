BINARY  := terraform-provider-omv
VERSION := 0.1.0
OS_ARCH := $(shell go env GOOS)_$(shell go env GOARCH)
INSTALL_DIR := $(HOME)/.terraform.d/plugins/registry.terraform.io/sandy-rt/omv/$(VERSION)/$(OS_ARCH)

.PHONY: build install clean

build:
	go build -o $(BINARY) .

# Install into Terraform's local plugin directory so `terraform init` finds it.
install: build
	mkdir -p "$(INSTALL_DIR)"
	cp $(BINARY) "$(INSTALL_DIR)/$(BINARY)_v$(VERSION)"
	@echo "installed $(BINARY) v$(VERSION) -> $(INSTALL_DIR)"

clean:
	rm -f $(BINARY)
