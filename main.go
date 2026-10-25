package main

import (
	"context"
	"flag"
	"log"

	"github.com/blast-security/terraform-provider-blast/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// Set by goreleaser.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/blast-security/blast",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
