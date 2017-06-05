package main

import (
	"context"
	"log"

	cdp "github.com/knq/chromedp"
	"github.com/knq/chromedp/runner"
)

func main() {
	var err error

	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create chrome instance
	c, err := cdp.New(ctxt, cdp.WithRunnerOptions(runner.Proxy("127.0.0.1:1080")))
	if err != nil {
		log.Fatal(err)
	}

	// run task list
	var res []string
	err = c.Run(ctxt, cdp.Tasks{
		cdp.Navigate(`https://www.brank.as`),
		cdp.WaitVisible(`#footer`, cdp.ByID),
		cdp.Evaluate(`Object.keys(window);`, &res),
	})
	if err != nil {
		log.Fatal(err)
	}

	// shutdown chrome
	err = c.Shutdown(ctxt)
	if err != nil {
		log.Fatal(err)
	}

	// wait for chrome to finish
	err = c.Wait()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("window object keys: %v", res)
}
