package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"agentic-change-review-copilot/internal/testflow"
)

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type cliApp struct {
	client httpDoer
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func main() {
	app := cliApp{
		client: &http.Client{Timeout: 15 * time.Second},
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
	os.Exit(app.run(os.Args[1:]))
}

func (app cliApp) run(args []string) int {
	if len(args) == 0 {
		writeUsage(app.stderr)
		return 2
	}

	switch args[0] {
	case "create":
		return app.runCreate(args[1:])
	case "get":
		return app.runGet(args[1:])
	case "report":
		return app.runReport(args[1:])
	case "help", "-h", "--help":
		writeUsage(app.stderr)
		return 0
	default:
		fmt.Fprintf(app.stderr, "unknown command %q\n\n", args[0])
		writeUsage(app.stderr)
		return 2
	}
}

func (app cliApp) runCreate(args []string) int {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	flags.SetOutput(app.stderr)

	baseURL := flags.String("base-url", defaultBaseURL(), "Base URL of the running testflow API")
	filePath := flags.String("file", "", "Path to a CreateTestTaskRequest JSON file; defaults to stdin")
	wait := flags.Bool("wait", false, "Fetch task detail after the create call succeeds")

	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(app.stderr, "create does not accept positional arguments")
		return 2
	}

	payload, err := app.readInput(*filePath)
	if err != nil {
		fmt.Fprintf(app.stderr, "read create payload: %v\n", err)
		return 1
	}

	body, _, err := app.doRequest(http.MethodPost, strings.TrimRight(*baseURL, "/")+"/api/v1/test-tasks", "application/json", bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(app.stderr, "create task: %v\n", err)
		return 1
	}

	if !*wait {
		if err := writeFormattedBody(app.stdout, body, "application/json"); err != nil {
			fmt.Fprintf(app.stderr, "write response: %v\n", err)
			return 1
		}
		return 0
	}

	var created testflow.CreateTestTaskResponse
	if err := json.Unmarshal(body, &created); err != nil {
		fmt.Fprintf(app.stderr, "decode create response: %v\n", err)
		return 1
	}
	if created.TaskID == "" {
		fmt.Fprintln(app.stderr, "create response did not include task_id")
		return 1
	}

	detailURL := strings.TrimRight(*baseURL, "/") + "/api/v1/test-tasks/" + url.PathEscape(created.TaskID)
	detailBody, _, err := app.doRequest(http.MethodGet, detailURL, "", nil)
	if err != nil {
		fmt.Fprintf(app.stderr, "get task detail: %v\n", err)
		return 1
	}
	if err := writeFormattedBody(app.stdout, detailBody, "application/json"); err != nil {
		fmt.Fprintf(app.stderr, "write detail response: %v\n", err)
		return 1
	}
	return 0
}

func (app cliApp) runGet(args []string) int {
	flags := flag.NewFlagSet("get", flag.ContinueOnError)
	flags.SetOutput(app.stderr)

	baseURL := flags.String("base-url", defaultBaseURL(), "Base URL of the running testflow API")
	taskID := flags.String("task-id", "", "Task identifier to fetch")

	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *taskID == "" {
		fmt.Fprintln(app.stderr, "get requires -task-id")
		return 2
	}

	body, _, err := app.doRequest(http.MethodGet, strings.TrimRight(*baseURL, "/")+"/api/v1/test-tasks/"+url.PathEscape(*taskID), "", nil)
	if err != nil {
		fmt.Fprintf(app.stderr, "get task: %v\n", err)
		return 1
	}
	if err := writeFormattedBody(app.stdout, body, "application/json"); err != nil {
		fmt.Fprintf(app.stderr, "write response: %v\n", err)
		return 1
	}
	return 0
}

func (app cliApp) runReport(args []string) int {
	flags := flag.NewFlagSet("report", flag.ContinueOnError)
	flags.SetOutput(app.stderr)

	baseURL := flags.String("base-url", defaultBaseURL(), "Base URL of the running testflow API")
	taskID := flags.String("task-id", "", "Task identifier to export")
	format := flags.String("format", "markdown", "Report format: markdown or json")

	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *taskID == "" {
		fmt.Fprintln(app.stderr, "report requires -task-id")
		return 2
	}

	reportURL := strings.TrimRight(*baseURL, "/") + "/api/v1/test-tasks/" + url.PathEscape(*taskID) + "/report?format=" + url.QueryEscape(*format)
	body, contentType, err := app.doRequest(http.MethodGet, reportURL, "", nil)
	if err != nil {
		fmt.Fprintf(app.stderr, "export report: %v\n", err)
		return 1
	}

	if err := writeFormattedBody(app.stdout, body, contentType); err != nil {
		fmt.Fprintf(app.stderr, "write report: %v\n", err)
		return 1
	}
	return 0
}

func (app cliApp) doRequest(method, requestURL, contentType string, body io.Reader) ([]byte, string, error) {
	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, "", err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := app.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("%s %s returned %d: %s", method, requestURL, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return respBody, resp.Header.Get("Content-Type"), nil
}

func (app cliApp) readInput(filePath string) ([]byte, error) {
	var (
		body []byte
		err  error
	)
	if filePath != "" {
		body, err = os.ReadFile(filePath)
	} else {
		body, err = io.ReadAll(app.stdin)
	}
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("empty input")
	}
	return body, nil
}

func defaultBaseURL() string {
	if value := strings.TrimSpace(os.Getenv("TESTFLOW_API_BASE_URL")); value != "" {
		return value
	}
	return "http://localhost:8080"
}

func writeFormattedBody(w io.Writer, body []byte, contentType string) error {
	if looksLikeJSON(contentType, body) {
		var out bytes.Buffer
		if err := json.Indent(&out, body, "", "  "); err == nil {
			out.WriteByte('\n')
			_, err = w.Write(out.Bytes())
			return err
		}
	}

	if len(body) > 0 {
		if _, err := w.Write(body); err != nil {
			return err
		}
	}
	if len(body) == 0 || body[len(body)-1] != '\n' {
		_, err := fmt.Fprintln(w)
		return err
	}
	return nil
}

func looksLikeJSON(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "json") {
		return true
	}
	trimmed := bytes.TrimSpace(body)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}

func writeUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  testflow-cli create [-base-url URL] [-file request.json] [-wait]")
	fmt.Fprintln(w, "  testflow-cli get -task-id ID [-base-url URL]")
	fmt.Fprintln(w, "  testflow-cli report -task-id ID [-format markdown|json] [-base-url URL]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Environment:")
	fmt.Fprintln(w, "  TESTFLOW_API_BASE_URL  Default API base URL when -base-url is omitted")
}
