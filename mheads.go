package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func worker(id int, jobs <-chan requestJob, results chan<- requestResults, wg *sync.WaitGroup, client *http.Client) {

	for j := range jobs {

		// notify start
		fmt.Println("Sending: ", j.Method, j.Location)

		jobResult := requestResults{
			jobPosition: j.JobPosition,
			Location:    j.Location,
			Request:     "",
			Result:      "",
		}

		// request
		request, err := http.NewRequest(j.Method, j.Location, nil)
		if err != nil {
			jobResult.Request = err.Error()
			jobResult.Status = "FAIL"
		} else {
			jobResult.Request = fmt.Sprintf("%s %s", strings.ToUpper(j.Method), j.Location)
			resp, err := client.Do(request)
			if err != nil {
				jobResult.Result = err.Error()
				jobResult.Status = "FAIL"
			} else {
				dump, err := httputil.DumpResponse(resp, true)
				if err != nil {
					panic(err)
				}
				jobResult.Result = fmt.Sprintf("%s", dump)
				jobResult.Status = "SUCCESS"
			}
		}

		time.Sleep(time.Second)
		results <- jobResult
	}
	wg.Done()
}

func main() {

	var needJson bool
	var pool int
	var method string
	var ua string

	flag.BoolVar(&needJson, "json", false, "true to output in json")
	flag.IntVar(&pool, "workers", 10, "concurrent workers to spawn")
	flag.StringVar(&method, "method", "HEAD", "valid HTTP method")
	flag.StringVar(&ua, "agent", "Mozilla/4.0 (compatible; MSIE 6.0; Windows NT 5.1; FSL 7.0.6.01001)", "user agent string")

	flag.Parse()

	args := flag.Args()
	var file string
	if len(os.Args) > 1 {
		file = fmt.Sprintf(strings.Join(args, " "))
	} else {
		file = "domains.txt"
	}

	// get the jobs list
	urls := getTheFileJobs(file, method, ua)

	// Setup the queueueueues
	jobs := make(chan requestJob, len(urls))
	results := make(chan requestResults, len(urls))

	// setup the client
	client := &http.Client{
		Timeout: time.Second * 10,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// populate the worker pool
	var wg sync.WaitGroup
	for w := 1; w <= pool; w++ {
		wg.Add(1)
		go worker(w, jobs, results, &wg, client)
	}

	// seed the job pool
	for _, entry := range urls {
		jobs <- entry
	}
	close(jobs)

	// wait for all worker to finish and close the channel
	wg.Wait()
	close(results)

	var r []requestResults
	for res := range results {
		r = append(r, res)
	}

	sort.Slice(r, func(i, j int) bool {
		return r[i].jobPosition < r[j].jobPosition
	})

	// output the results
	if needJson {
		output, err := json.MarshalIndent(r, "", "    ")
		if err != nil {
			panic(err)
		}

		fmt.Printf("%s", output)
	} else {
		// LOL Formatting
		fmt.Println("\n")
		for _, res := range r {
			fmt.Printf("%s [%s]\n", res.Request, res.Status)
			fmt.Println("--------")
			fmt.Println(res.Result)
			if res.Status == "FAIL" {
				fmt.Println("\n")
			}
		}
	}

}

func getTheFileJobs(path string, method string, ua string) (jobs []requestJob) {

	pos := 0

	// open it
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close() // kill it

	// read it
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		jobs = append(jobs, requestJob{
			JobPosition: pos,
			Location:    "http://" + scanner.Text(),
			Method:      method,
			UserAgent:   ua,
		})
		jobs = append(jobs, requestJob{
			JobPosition: pos + 1,
			Location:    "https://" + scanner.Text(),
			Method:      method,
			UserAgent:   ua,
		})
		pos = pos + 2
	}

	return jobs
}

type requestJob struct {
	JobPosition int
	Location    string
	Method      string
	UserAgent   string
}

type requestResults struct {
	jobPosition int
	Location    string
	Status      string
	Request     string
	Result      string
}
