package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

func worker(id int, jobs <-chan requestJob, results chan<- requestResults, wg *sync.WaitGroup, client *http.Client) {

	for j := range jobs {

		// notify start
		fmt.Println("=> ", j.Method, j.URL.String())

		jr := requestResults{
			jobPosition:    j.JobPosition,
			Proto:          j.URL.Scheme,
			Domain:         j.URL.Host,
			Path:           j.URL.RequestURI(),
			RequestMethod:  j.Method,
			RequestFull:    "",
			mhResultStatus: "",
			ResponseStatus: "",
			ResponseFull:   "",
		}

		// request
		request, err := http.NewRequest(j.Method, j.URL.String(), nil)
		if err != nil {
			jr.mhResultStatus = "FAIL: " + err.Error()
		} else {
			jr.RequestFull = dumpRequest(request)
			resp, err := client.Do(request)
			if err != nil {
				jr.mhResultStatus = "FAIL: " + err.Error()
			} else {
				jr.ResponseStatus = fmt.Sprintf("%d", resp.StatusCode)
				jr.ResponseFull = fmt.Sprintf("%s", dumpResponse(resp))
				jr.mhResultStatus = "SUCCESS"
			}
		}

		// notify end
		if jr.mhResultStatus == "SUCCESS" {
			fmt.Println("<= ", j.URL.String(), jr.ResponseStatus)
		} else {
			fmt.Println("[!] ", j.URL.String(), jr.mhResultStatus)
		}


		time.Sleep(time.Second)
		results <- jr
	}
	wg.Done()
}

func dumpResponse(resp *http.Response) (string) {
	dump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		panic(err)
	}
	return string(dump)
}

func dumpRequest(request *http.Request) (string) {
	requestDump, err := httputil.DumpRequest(request, true)
	if err != nil {
		panic(err)
	}
	return string(requestDump)
}

func main() {

	var needJson bool
	var needGrep bool
	var needFile bool
	var needStd bool
	var outputAll string
	var pool int
	var method string
	var ua string

	flag.BoolVar(&needJson, "oJ", false, "output in json")
	flag.BoolVar(&needGrep, "oG", false, "output in greppable format")
	flag.BoolVar(&needFile, "oF", false, "save each result to a file with raw format")
	flag.BoolVar(&needStd, "oS", false, "output in a crappy format")
	flag.StringVar(&outputAll, "oA", "", "output all formats")
	flag.IntVar(&pool, "w", 10, "concurrent workers to spawn")
	flag.StringVar(&method, "m", "HEAD", "valid HTTP method")
	flag.StringVar(&ua, "a", "Mozilla/4.0 (compatible; MSIE 6.0; Windows NT 5.1; FSL 7.0.6.01001)", "user agent string")

	flag.Parse()

	outputNames := "mhoutput"
	if len(outputAll) > 0 {
		fmt.Println("using default output file names")
		outputNames = outputAll
	}

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
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout: time.Second * 10,
		Transport: tr,
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
	if needJson || outputAll != "" {
		output, err := json.MarshalIndent(r, "", "    ")
		if err != nil {
			panic(err)
		}

		jsonText := fmt.Sprintf("%s", output)
		basicFileOutputOrOverwrite([]byte(jsonText), outputNames + ".json")
	}
	if needGrep || outputAll != "" {
		grepOutput := ""
		for _, res := range r {
			grepOutput += fmt.Sprintf("%s\n", getGreppable(res))
		}
		basicFileOutputOrOverwrite([]byte(grepOutput), outputNames + ".grep")
	}
	if needFile {
		// make the folder
		if _, err := os.Stat(fmt.Sprintf("%s.manyheads-files", outputNames)); os.IsNotExist(err) {
			os.Mkdir(fmt.Sprintf("%s.manyheads-files", outputNames), os.ModePerm)
		}

		for _, res := range r {
			standardOutput := crappyOutputFormat(res)
			err := basicFileOutputOrOverwrite([]byte(standardOutput), fmt.Sprintf("%s.manyheads-files/%s.%s.mhdata", outputNames, res.Domain, res.Proto))
			if err != nil {
				fmt.Println(err)
			}
		}
	}
	if needStd || outputAll != "" {
		// LOL, Formatting
		standardOutput := ""
		for _, res := range r {
			standardOutput += crappyOutputFormat(res)
		}
		basicFileOutputOrOverwrite([]byte(standardOutput), outputNames + ".manyheads")
	}
}

func crappyOutputFormat(res requestResults) (string) {
	outstring := ""
	outstring += fmt.Sprintf("///////////////////////////////\n%s %s => %s [%s]\n", res.RequestMethod, res.Domain, res.ResponseStatus, res.mhResultStatus)
	outstring += fmt.Sprintf("--------\n")
	outstring += fmt.Sprintf("%s%s\n", res.RequestFull, res.ResponseFull)
	return outstring
}


func basicFileOutputOrOverwrite(content []byte, path string) (err error) {

	// open output file
	fo, err := os.Create(path)
	if err != nil {
		return err
	}
	// close fo on exit and check for its returned error
	defer func() {
		if closeErr := fo.Close(); closeErr != nil {
			// if an error ocurred closing the file
			// but not folling another err, return the value
			if err == nil {
				err = closeErr
			}
		}
	}()

	// write all lines
	if _, err := fo.Write(content); err != nil {
		return err
	}
	return nil
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

		for _,url := range importLocation(scanner.Text()) {
			jobs = append(jobs, requestJob{
				JobPosition: pos,
				URL:         url,
				Method:      method,
				UserAgent:   ua,
			})
			pos++
		}
}

	return jobs
}

func importLocation(input string) ([]*url.URL) {

	var output []*url.URL
	if strings.HasPrefix(input, "http") {
		parsed, err := url.Parse(input)
		if err != nil {
			panic(err)
		}
		output = append(output, parsed)
	} else {
		parsedHTTP, err := url.Parse("http://" + input)
		if err != nil {
			panic(err)
		}
		parsedHTTPS, err := url.Parse("https://" + input)
		if err != nil {
			panic(err)
		}
		output = append(output, parsedHTTP)
		output = append(output, parsedHTTPS)
	}
	return output
}

type requestJob struct {
	JobPosition int
	URL         *url.URL
	Method      string
	UserAgent   string
}

type requestResults struct {
	// for output ordering
	jobPosition int

	// url info
	Proto 		string
	Domain      string
	Path        string

	// request info
	RequestMethod string
	RequestFull   string

	// tool info
	mhResultStatus string

	// result info
	ResponseStatus string
	ResponseFull         string
}

func getGreppable(results requestResults) (string) {
	//domain//proto//mhStatus//requestMethod//ResponseStatus//proto+domain+path//requestfull//ResponseFull
	grepout := fmt.Sprintf(
		"%s // %s // %s // %s // %s // %s // %s // %s //",
		results.Domain,
		results.RequestMethod,
		results.Proto,
		results.mhResultStatus,
		results.ResponseStatus,
		results.Proto + "://" + results.Domain + results.Path,
		singleLineOutput(results.RequestFull),
		singleLineOutput(results.ResponseFull),
		)
	return grepout
}

func singleLineOutput(content string) (string) {
	re := regexp.MustCompile(`\r?\n`)
	return re.ReplaceAllString(content, "\\n")
}