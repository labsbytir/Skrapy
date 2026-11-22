package main

import (
	"fmt"
	"os"

	"github.com/labsbytir/Skrapy/internal/proxy"
)

func main() {
	urls, err := proxy.ResolveWebshare()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(urls) == 0 {
		fmt.Fprintln(os.Stderr, "no proxies returned")
		os.Exit(1)
	}
	fmt.Println(urls[0])
}