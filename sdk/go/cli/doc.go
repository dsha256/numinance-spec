// Package cli is the command logic of the numi CLI: subcommand dispatch,
// "numi validate" and the stable exit-code table. It is public so that the
// open build of numi and the engine build run the same code; the engine build
// injects the Resolver that needs the private profile context. The package
// depends on the standard library and the spec SDK only.
package cli
