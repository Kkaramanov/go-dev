package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type config struct {
	From    int
	To      int
	Workers int
	Timeout int
}

func validateFlags(cnf config) (bool, string) {
	if cnf.From <= 0 && cnf.To <= 0 && cnf.Workers <= 0 {
		return false, "--from --to --workers must be graater then zero"
	}
	if cnf.Timeout < 0 {
		return false, "--timeout must be greater then zero"
	}
	if cnf.From > cnf.To {
		return false, "--from must be smoller then --to"
	}

	return true, ""
}

func main() {
	var cnf config

	flag.IntVar(&cnf.From, "from", 0, "First film id (must be > 0)")
	flag.IntVar(&cnf.To, "to", 0, "Last film id")
	flag.IntVar(&cnf.Workers, "workers", 10, "Worker pulls nums")
	flag.IntVar(&cnf.Timeout, "timeout", 5, "Timeout")

	flag.Parse()

	ok, msg := validateFlags(cnf)
	if !ok {
		fmt.Println(msg)
		os.Exit(2)
	}

	client := &http.Client{Timeout: time.Duration(cnf.Timeout) * time.Second}
	worker := NewWorker(client, "https://homeworksite.site")

	pool := NewWorkerPool(cnf.Workers, 10, worker)

	ctx, cencel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cencel()

	pool.Start(ctx)

	go func() {
		defer pool.Stop()
		for i := cnf.From; i <= cnf.To; i++ {
			select {
			case <-ctx.Done():
				return
			case pool.jobs <- i:
			}
		}
	}()

	for res := range pool.results {
		if res.Err != nil {
			if errors.Is(res.Err, context.Canceled) {
				continue
			}
			fmt.Printf("Error Job %d faild: %v\n", res.JobID, res.Err)
			continue
		}

		fmt.Printf("%v - %v - %v - %v\n", res.Response.Id, res.Response.Title, res.Response.Year, res.Response.Director)
	}

}
