// llocal-survey reads a project folder and prints its survey.json, or with -plan the plan
// llocal shows the builder. It only reads the folder; it never runs anything in it.
//
//	llocal-survey [-plan [-url https://inventory.acme.llocal.app] [-ip 203.0.113.7]] [dir]
package main

import (
	"cmp"
	"encoding/json"
	"flag"
	"log"
	"os"

	"llocal.app/kit/survey"
)

func main() {
	plan := flag.Bool("plan", false, "print the plan made from the survey instead of the survey")
	url := flag.String("url", "", "the app's address on llocal, for the plan")
	ip := flag.String("ip", "", "llocal's outbound IP address, for the plan")
	flag.Parse()

	s, err := survey.Run(cmp.Or(flag.Arg(0), "."))
	if err != nil {
		log.Fatal(err)
	}
	var out any = s
	if *plan {
		out = s.Plan(*url, *ip)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		log.Fatal(err)
	}
}
