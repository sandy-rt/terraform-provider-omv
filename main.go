package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/sandy-rt/terraform-provider-omv/internal/provider"
)

// providerAddress is the canonical registry address for this provider.
const providerAddress = "registry.terraform.io/sandy-rt/omv"

// These are set by GoReleaser via -ldflags at build time.
var (
	version = "dev"
	commit  = "" //nolint:unused // injected via ldflags
)

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: providerAddress,
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
