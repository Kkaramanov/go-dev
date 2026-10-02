package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

type FilmDescriptionResponse struct {
	Id       int    `json:"id"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Director string `json:"director"`
}

type Worker struct {
	client  *http.Client
	baseURL string
}

type Result struct {
	JobID    int
	Response FilmDescriptionResponse
	Err      error
}

type WorkerPool struct {
	workerCount int
	worker      *Worker
	jobs        chan int
	results     chan Result
	wg          sync.WaitGroup
}

func NewWorker(client *http.Client, baseURL string) *Worker {
	return &Worker{client: client, baseURL: baseURL}
}

func NewWorkerPool(workerCount int, bufferSize int, worker *Worker) *WorkerPool {
	return &WorkerPool{
		workerCount: workerCount,
		worker:      worker,
		jobs:        make(chan int, bufferSize),
		results:     make(chan Result, bufferSize),
	}
}

func (w *Worker) Process(ctx context.Context, job int) Result {
	targetURL, err := url.JoinPath(w.baseURL, strconv.Itoa(job), "info.0.json")
	if err != nil {
		return Result{JobID: job, Err: fmt.Errorf("build url: %w", err)}
	}

	reqCtx, cencel := context.WithTimeout(ctx, w.client.Timeout)
	defer cencel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return Result{JobID: job, Err: fmt.Errorf("build request: %w", err)}
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return Result{JobID: job, Err: fmt.Errorf("do request: %w", err)}
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return Result{JobID: job, Err: fmt.Errorf("bad status: %d", resp.StatusCode)}
	}

	var respInfo FilmDescriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&respInfo); err != nil {
		return Result{JobID: job, Err: fmt.Errorf("decode json: %w", err)}
	}

	return Result{JobID: job, Response: respInfo}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	for i := 0; i < wp.workerCount; i++ {
		wp.wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-wp.jobs:
					if !ok {
						return
					}
					res := wp.worker.Process(ctx, job)

					select {
					case <-ctx.Done():
						return
					case wp.results <- res:
					}

				}
			}
		})
	}
}

func (wp *WorkerPool) Stop() {
	close(wp.jobs)
	wp.wg.Wait()
	close(wp.results)
}
