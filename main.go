package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)
// ## this is done my me max
const (
	maxRetries  = 5
	httpTimeout = 120 * time.Second
	userAgent   = "crtsh-recon/1.0"
)

type crtEntry struct {
	NameValue string `json:"name_value"`
}

var client = &http.Client{Timeout: httpTimeout}

func query(domain string) ([]string, error) {
	url := "https://crt.sh/?q=%25." + domain + "&output=json"

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		var entries []crtEntry
		if err := json.Unmarshal(body, &entries); err != nil {
			lastErr = fmt.Errorf("attempt %d: invalid JSON: %v", attempt, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		seen := make(map[string]struct{})
		var results []string
		for _, e := range entries {
			for _, name := range strings.Split(e.NameValue, "\n") {
				name = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "*.")))
				if name == "" {
					continue
				}
				if _, ok := seen[name]; !ok {
					seen[name] = struct{}{}
					results = append(results, name)
				}
			}
		}
		return results, nil
	}

	return nil, fmt.Errorf("all %d attempts failed for %s: %v", maxRetries, domain, lastErr)
}

func isValidDomain(d string) bool {
	return strings.Contains(d, ".") &&
		!strings.HasPrefix(d, "http://") &&
		!strings.HasPrefix(d, "https://")
}

func main() {
	var output string

	args := os.Args[1:]
	var remaining []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-o" && i+1 < len(args) {
			output = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "-o=") {
			output = strings.TrimPrefix(args[i], "-o=")
		} else if args[i] == "-h" || args[i] == "--help" {
			fmt.Println("Usage: crtsh [-o output] <domain>")
			fmt.Println("       echo 'domain.com' | crtsh [-o output]")
			os.Exit(0)
		} else {
			remaining = append(remaining, args[i])
		}
	}

	var domain string

	// stdin pipe
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		sc := bufio.NewScanner(os.Stdin)
		if sc.Scan() {
			domain = strings.TrimSpace(sc.Text())
		}
	}

	// positional arg overrides stdin
	if len(remaining) > 0 {
		domain = remaining[0]
	}

	if domain == "" {
		fmt.Println("Usage: crtsh [-o output] <domain>")
		fmt.Println("       echo 'domain.com' | crtsh [-o output]")
		os.Exit(1)
	}

	if !isValidDomain(domain) {
		fmt.Fprintf(os.Stderr, "invalid domain: %s\n", domain)
		os.Exit(1)
	}

	results, err := query(domain)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error [%s]: %v\n", domain, err)
		os.Exit(1)
	}

	out := os.Stdout
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		defer f.Close()
		out = f
	}

	writer := bufio.NewWriter(out)
	defer writer.Flush()

	for _, r := range results {
		fmt.Fprintln(writer, r)
	}
}
