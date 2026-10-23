package cli

import (
	"github.com/kinxyo/zz-go/internal/network"
	"github.com/kinxyo/zz-go/internal/terminal"
)

func Run(args []string, flags *terminal.Flags) {

	var m, p string = extract(args)

	var method network.Method = network.ParseMethod(m)
	var url network.URL = network.ParseURL(p, *flags.ForceHttp)
	var payload network.Payload = network.ParsePayload(&method, args)

	res, err := network.CURL(method, url, &payload)
	if err != nil {
		terminal.Fatal("%s\n", err)
	}

	if *flags.Verbose {
		res.Print(true, true, true, *flags.NoFormat)
		return
	}

	res.Print(*flags.PrintTime, *flags.PrintURL, *flags.PrintStatus, *flags.NoFormat)
}
