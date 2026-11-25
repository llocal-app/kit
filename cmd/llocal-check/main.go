// llocal-check says whether an app is in llocal's standard shape. It exits 1 if not.
//
//	llocal-check [-survey survey.json] [-json] [-built] [dir]
//
// llocal's build runs llocal-check -finish on what it built, which changes the folder: a
// standard app's node_modules goes, and each link becomes what it points to (see
// check.Finish). It prints why and exits 1 when a link can't be kept.
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"llocal.app/kit/check"
)

func main() {
	surveyFile := flag.String("survey", "", "survey.json from llocal's survey; without it only / is requested")
	asJSON := flag.Bool("json", false, "print the report as JSON")
	built := flag.Bool("built", false, "the app is built already: skip install and build")
	finish := flag.Bool("finish", false, "do to a folder llocal has built what it does before it keeps it (this changes the folder), and check nothing")
	flag.Parse()
	if *finish {
		if err := check.Finish(cmp.Or(flag.Arg(0), ".")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	var survey check.Survey
	if *surveyFile != "" {
		var err error
		if survey, err = check.LoadSurvey(*surveyFile); err != nil {
			log.Fatal(err)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	run := check.Run
	if *built {
		run = check.Built
	}
	report := run(ctx, cmp.Or(flag.Arg(0), "."), survey)

	if *asJSON {
		json.NewEncoder(os.Stdout).Encode(report)
	} else {
		for _, r := range report.Checks {
			if r.OK {
				fmt.Printf("ok    %s\n", r.Name)
			} else {
				fmt.Printf("FAIL  %s: %s\n", r.Name, r.Reason)
			}
		}
		if report.IdleMemory > 0 {
			fmt.Printf("idle memory: %.1f MB\n", float64(report.IdleMemory)/(1<<20))
		}
		if report.StartMillis > 0 {
			fmt.Printf("start: %d ms, disk: %.1f MB\n", report.StartMillis, float64(report.DiskBytes)/(1<<20))
		}
	}
	if !report.OK {
		os.Exit(1)
	}
}
