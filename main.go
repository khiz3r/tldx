package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── TLD LIST ────────────────────────────────────────────────────────────────

var tlds = []string{
	// Country codes
	"ac", "ad", "ae", "af", "ag", "ai", "al", "am", "ao", "aq", "ar", "as",
	"at", "au", "aw", "ax", "az", "ba", "bb", "bd", "be", "bf", "bg", "bh",
	"bi", "bj", "bm", "bn", "bo", "br", "bs", "bt", "bv", "bw", "by", "bz",
	"ca", "cc", "cd", "cf", "cg", "ch", "ci", "ck", "cl", "cm", "cn", "co",
	"cr", "cu", "cv", "cw", "cx", "cy", "cz", "de", "dj", "dk", "dm", "do",
	"dz", "ec", "ee", "eg", "er", "es", "et", "eu", "fi", "fj", "fk", "fm",
	"fo", "fr", "ga", "gb", "gd", "ge", "gf", "gg", "gh", "gi", "gl", "gm",
	"gn", "gp", "gq", "gr", "gs", "gt", "gu", "gw", "gy", "hk", "hm", "hn",
	"hr", "ht", "hu", "id", "ie", "il", "im", "in", "io", "iq", "ir", "is",
	"it", "je", "jm", "jo", "jp", "ke", "kg", "kh", "ki", "km", "kn", "kp",
	"kr", "kw", "ky", "kz", "la", "lb", "lc", "li", "lk", "lr", "ls", "lt",
	"lu", "lv", "ly", "ma", "mc", "md", "me", "mg", "mh", "mk", "ml", "mm",
	"mn", "mo", "mp", "mq", "mr", "ms", "mt", "mu", "mv", "mw", "mx", "my",
	"mz", "na", "nc", "ne", "nf", "ng", "ni", "nl", "no", "np", "nr", "nu",
	"nz", "om", "pa", "pe", "pf", "pg", "ph", "pk", "pl", "pm", "pn", "pr",
	"ps", "pt", "pw", "py", "qa", "re", "ro", "rs", "ru", "rw", "sa", "sb",
	"sc", "sd", "se", "sg", "sh", "si", "sj", "sk", "sl", "sm", "sn", "so",
	"sr", "ss", "st", "su", "sv", "sx", "sy", "sz", "tc", "td", "tf", "tg",
	"th", "tj", "tk", "tl", "tm", "tn", "to", "tr", "tt", "tv", "tw", "tz",
	"ua", "ug", "uk", "us", "uy", "uz", "va", "vc", "ve", "vg", "vi", "vn",
	"vu", "wf", "ws", "ye", "yt", "za", "zm", "zw",
	// Generic
	"com", "net", "org", "info", "biz", "app", "dev", "io", "co",
	// Second-level + country combos (high value)
	"com.au", "com.br", "com.cn", "com.co", "com.mx", "com.ar",
	"com.tr", "com.pe", "com.ve", "com.bo", "com.ec", "com.py",
	"com.uy", "com.gt", "com.pa", "com.do", "com.hn", "com.ni",
	"com.sv", "com.cr", "com.cu", "com.pr", "com.pk", "com.bd",
	"com.np", "com.lk", "com.sg", "com.ph", "com.vn", "com.my",
	"com.id", "com.hk", "com.tw", "com.kh", "com.mm", "com.la",
	"com.kw", "com.sa", "com.ae", "com.qa", "com.bh", "com.om",
	"com.jo", "com.lb", "com.eg", "com.ly", "com.tn", "com.ma",
	"com.dz", "com.gh", "com.ng", "com.ke", "com.tz", "com.ug",
	"com.et", "com.zm", "com.zw", "com.na", "com.bw",
	"co.uk", "co.in", "co.nz", "co.za", "co.jp", "co.kr",
	"co.ke", "co.tz", "co.ug", "co.zw", "co.zm",
	"org.uk", "org.au", "org.in", "net.au", "net.in",
}

// loadTLDs reads a TLD wordlist file (one entry per line, # comments ignored).
// Returns the loaded slice and an error. Blank lines and leading dots are handled.
func loadTLDs(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []string
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip leading dot if present (e.g. ".com" → "com")
		line = strings.TrimPrefix(line, ".")
		line = strings.ToLower(line)
		if !seen[line] {
			seen[line] = true
			out = append(out, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ─── USER AGENTS ─────────────────────────────────────────────────────────────

var userAgentsByType = map[string][]string{
	"chrome": {
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 Mobile Safari/537.36",
	},
	"firefox": {
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:125.0) Gecko/20100101 Firefox/125.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14.4; rv:125.0) Gecko/20100101 Firefox/125.0",
		"Mozilla/5.0 (X11; Linux x86_64; rv:125.0) Gecko/20100101 Firefox/125.0",
		"Mozilla/5.0 (Android 14; Mobile; rv:125.0) Gecko/125.0 Firefox/125.0",
	},
	"safari": {
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_4_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (iPad; CPU OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
	},
}

var allUserAgents []string

func init() {
	for _, agents := range userAgentsByType {
		allUserAgents = append(allUserAgents, agents...)
	}
}

func pickUA(uaType string) string {
	if pool, ok := userAgentsByType[uaType]; ok {
		return pool[rand.Intn(len(pool))]
	}
	return allUserAgents[rand.Intn(len(allUserAgents))]
}


// ─── STATUS CODE PARSING ──────────────────────────────────────────────────────

type statusFilter struct {
	codes  map[int]bool
	ranges [][2]int
}

func parseStatusCodes(raw string) *statusFilter {
	sf := &statusFilter{codes: make(map[int]bool)}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, err1 := strconv.Atoi(bounds[0])
			hi, err2 := strconv.Atoi(bounds[1])
			if err1 == nil && err2 == nil {
				sf.ranges = append(sf.ranges, [2]int{lo, hi})
			}
		} else {
			code, err := strconv.Atoi(part)
			if err == nil {
				sf.codes[code] = true
			}
		}
	}
	return sf
}

func (sf *statusFilter) match(code int) bool {
	if sf.codes[code] {
		return true
	}
	for _, r := range sf.ranges {
		if code >= r[0] && code <= r[1] {
			return true
		}
	}
	return false
}

// ─── CONFIG ───────────────────────────────────────────────────────────────────

type Config struct {
	depth      int // 0 = unlimited
	threads    int
	outputFile string
	silent     bool
	timeout    time.Duration
	statusRaw  string
	filter     *statusFilter
	userAgent  string
	uaType     string // chrome | firefox | safari | random
}

// ─── RESULT ───────────────────────────────────────────────────────────────────

type Result struct {
	url      string
	status   int
	title    string
	redirect string
}

// ─── HTTP CLIENT ──────────────────────────────────────────────────────────────

func makeClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives:   true,
			MaxIdleConnsPerHost: 0,
		},
	}
}

func extractTitle(body string) string {
	lower := strings.ToLower(body)
	start := strings.Index(lower, "<title>")
	if start == -1 {
		return ""
	}
	start += 7
	end := strings.Index(lower[start:], "</title>")
	if end == -1 {
		return ""
	}
	title := strings.TrimSpace(body[start : start+end])
	// Truncate long titles
	if len(title) > 60 {
		title = title[:57] + "..."
	}
	return title
}

func probe(client *http.Client, domain string, ua string) (int, string, string) {
	for _, scheme := range []string{"https", "http"} {
		url := scheme + "://" + domain
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		// Capture redirect location before reading body
		redirect := ""
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			redirect = resp.Header.Get("Location")
		}

		// Read limited body for title extraction
		buf := make([]byte, 4096)
		n, _ := resp.Body.Read(buf)
		resp.Body.Close()

		title := extractTitle(string(buf[:n]))
		return resp.StatusCode, title, redirect
	}
	return 0, "", ""
}

// ─── COLORS ───────────────────────────────────────────────────────────────────

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

func colorStatus(code int) string {
	s := strconv.Itoa(code)
	switch {
	case code >= 200 && code < 300:
		return colorGreen + s + colorReset
	case code >= 300 && code < 400:
		return colorCyan + s + colorReset
	case code == 401 || code == 403:
		return colorYellow + s + colorReset
	case code >= 400:
		return colorRed + s + colorReset
	default:
		return colorGray + s + colorReset
	}
}

// ─── WILDCARD EXPANSION ───────────────────────────────────────────────────────

// expandWildcard handles patterns with * in them:
//   social.zoho.*     → social.zoho.com, social.zoho.cn, ... (replace trailing .*)
//   *.zoho.com        → [no expansion — prefix wildcard, skip with warning]
//   social.zoho.com   → [no wildcard, return as-is for normal append mode]
//
// Returns (expanded []string, wasWildcard bool)
func expandWildcard(input string) ([]string, bool) {
	// Trailing wildcard: social.zoho.*
	if strings.HasSuffix(input, ".*") {
		base := strings.TrimSuffix(input, ".*")
		var expanded []string
		for _, tld := range tlds {
			expanded = append(expanded, base+"."+tld)
		}
		return expanded, true
	}

	// Leading wildcard: *.zoho.com — not supported for HTTP probing
	// (would need a subdomain wordlist), warn and skip
	if strings.HasPrefix(input, "*.") {
		fmt.Fprintf(os.Stderr, "%s[!] Prefix wildcard '*.zoho.com' not supported — use subfinder/amass first, then pipe results here%s\n",
			colorYellow, colorReset)
		return nil, true
	}

	// Mid wildcard: social.*.com — not supported
	if strings.Contains(input, "*") {
		fmt.Fprintf(os.Stderr, "%s[!] Unsupported wildcard pattern: %s — only trailing .* is supported%s\n",
			colorYellow, input, colorReset)
		return nil, true
	}

	return []string{input}, false
}

// ─── CORE ENGINE ─────────────────────────────────────────────────────────────

type Engine struct {
	cfg     *Config
	client  *http.Client
	mu      sync.Mutex
	seen    map[string]bool
	outFile *os.File
	results []Result
}

func NewEngine(cfg *Config) *Engine {
	e := &Engine{
		cfg:    cfg,
		client: makeClient(cfg.timeout),
		seen:   make(map[string]bool),
	}
	if cfg.outputFile != "" {
		f, err := os.Create(cfg.outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[!] Cannot create output file: %v\n", err)
		} else {
			e.outFile = f
		}
	}
	return e
}

func (e *Engine) Close() {
	if e.outFile != nil {
		e.outFile.Close()
	}
}

func (e *Engine) markSeen(domain string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.seen[domain] {
		return false
	}
	e.seen[domain] = true
	return true
}

func (e *Engine) printBanner() {
	if e.cfg.silent {
		return
	}
	fmt.Printf("\n%s%s tldx — TLD Permutation Bruteforcer%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", colorGray, colorReset)
	depth := strconv.Itoa(e.cfg.depth)
	if e.cfg.depth == 0 {
		depth = "unlimited"
	}
	fmt.Printf(" Depth    : %s\n", depth)
	fmt.Printf(" Threads  : %d\n", e.cfg.threads)
	fmt.Printf(" Status   : %s\n", e.cfg.statusRaw)
	fmt.Printf(" TLDs     : %d\n", len(tlds))
	if e.cfg.userAgent != "" {
		fmt.Printf(" UA       : custom\n")
	} else {
		fmt.Printf(" UA       : %s\n", e.cfg.uaType)
	}
	if e.cfg.outputFile != "" {
		fmt.Printf(" Output   : %s\n", e.cfg.outputFile)
	}
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", colorGray, colorReset)
}

func (e *Engine) record(r Result) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.results = append(e.results, r)
	if e.outFile != nil {
		line := fmt.Sprintf("[%d] %s", r.status, r.url)
		if r.title != "" {
			line += fmt.Sprintf(" (%s)", r.title)
		}
		if _, err := fmt.Fprintln(e.outFile, line); err != nil {
			fmt.Fprintf(os.Stderr, "[!] Failed to write to output file: %v\n", err)
		}
	}
}

func (e *Engine) printHit(r Result) {
	extra := ""
	if r.redirect != "" {
		extra = fmt.Sprintf(" %s(→ %s)%s", colorCyan, r.redirect, colorReset)
	} else if r.title != "" {
		extra = fmt.Sprintf(" %s(%s)%s", colorGray, r.title, colorReset)
	}
	fmt.Printf("[%s] %s%s%s%s\n",
		colorStatus(r.status),
		colorBold, r.url, colorReset,
		extra,
	)
}

// probeDirectly probes a list of already-concrete domains with no TLD appending.
// Used for wildcard-expanded inputs. Returns hits for further recursion.
func (e *Engine) probeDirectly(domains []string) []string {
	var wg sync.WaitGroup
	sem := make(chan struct{}, e.cfg.threads)
	var hitsMu sync.Mutex
	var hits []string

	for _, domain := range domains {
		wg.Add(1)
		sem <- struct{}{}

		go func(d string) {
			defer wg.Done()
			defer func() { <-sem }()

			ua := e.cfg.userAgent
			if ua == "" {
				ua = pickUA(e.cfg.uaType)
			}

			status, title, redirect := probe(e.client, d, ua)
			if status == 0 {
				return
			}

			if e.cfg.filter.match(status) {
				r := Result{url: d, status: status, title: title, redirect: redirect}
				e.record(r)
				if !e.cfg.silent {
					e.printHit(r)
				} else {
					fmt.Println(d)
				}

				hitsMu.Lock()
				hits = append(hits, d)
				hitsMu.Unlock()
			}
		}(domain)
	}

	wg.Wait()
	return hits
}

// processLevel probes all TLD variants of the given domains at this depth level
// Returns new hits to recurse into
func (e *Engine) processLevel(domains []string) []string {
	if len(domains) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, e.cfg.threads)
	var hitsMu sync.Mutex
	var hits []string

	for _, base := range domains {
		for _, tld := range tlds {
			candidate := base + "." + tld

			if !e.markSeen(candidate) {
				continue
			}

			wg.Add(1)
			sem <- struct{}{}

			go func(domain string) {
				defer wg.Done()
				defer func() { <-sem }()

				ua := e.cfg.userAgent
				if ua == "" {
					ua = pickUA(e.cfg.uaType)
				}

				status, title, redirect := probe(e.client, domain, ua)
				if status == 0 {
					return
				}

				if e.cfg.filter.match(status) {
					r := Result{url: domain, status: status, title: title, redirect: redirect}
					e.record(r)
					if !e.cfg.silent {
						e.printHit(r)
					} else {
						// silent mode: just print the URL
						fmt.Println(domain)
					}

					hitsMu.Lock()
					hits = append(hits, domain)
					hitsMu.Unlock()
				}
			}(candidate)
		}
	}

	wg.Wait()
	return hits
}

func (e *Engine) Run(inputs []string) {
	e.printBanner()

	// Split inputs into:
	//   wildcardExpanded — already concrete domains from patterns like social.zoho.*
	//   normalQueue      — concrete domains that will have TLDs appended recursively
	var wildcardExpanded []string
	var normalQueue []string

	for _, input := range inputs {
		expanded, wasWildcard := expandWildcard(input)
		if wasWildcard {
			// Mark all expanded domains as seen so recursive append won't re-probe them
			for _, d := range expanded {
				e.markSeen(d)
			}
			wildcardExpanded = append(wildcardExpanded, expanded...)
		} else {
			normalQueue = append(normalQueue, expanded...)
		}
	}

	// Phase 1: probe wildcard-expanded domains directly (no TLD appending)
	if len(wildcardExpanded) > 0 {
		if !e.cfg.silent {
			fmt.Printf("%s[~] Wildcard mode — probing %d expanded candidates directly%s\n",
				colorGray, len(wildcardExpanded), colorReset)
		}
		hits := e.probeDirectly(wildcardExpanded)
		// Hits from wildcard expansion feed into normal recursive append queue
		normalQueue = append(normalQueue, hits...)
	}

	// Phase 2: normal recursive TLD-append mode
	if len(normalQueue) > 0 {
		queue := normalQueue
		depth := 0

		for {
			depth++
			if e.cfg.depth > 0 && depth > e.cfg.depth {
				break
			}

			if !e.cfg.silent {
				fmt.Printf("%s[*] Depth %d — probing %d base(s) × %d TLDs = %d candidates%s\n",
					colorGray, depth, len(queue), len(tlds), len(queue)*len(tlds), colorReset)
			}

			hits := e.processLevel(queue)
			if len(hits) == 0 {
				break
			}
			queue = hits
		}
	}

	if !e.cfg.silent {
		fmt.Printf("\n%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", colorGray, colorReset)
		fmt.Printf("%s[✓] Done. %d hit(s) found.%s\n", colorGreen, len(e.results), colorReset)
		if e.cfg.outputFile != "" {
			fmt.Printf("%s[✓] Results saved to: %s%s\n", colorGreen, e.cfg.outputFile, colorReset)
		}
	}
}

// ─── MAIN ─────────────────────────────────────────────────────────────────────

func main() {
	depth := flag.Int("d", 1, "Recursion depth (0 = unlimited)")
	threads := flag.Int("t", 50, "Concurrent threads")
	output := flag.String("o", "", "Output file path")
	silent := flag.Bool("silent", false, "Silent mode — only print hits")
	timeoutSec := flag.Int("timeout", 10, "HTTP timeout in seconds")
	statusCodes := flag.String("sc", "200-299,301,302,403,401", "Status codes to treat as hits (e.g. 200-299,403,401)")
	inputList := flag.String("l", "", "File containing list of domains (one per line)")
	ua      := flag.String("ua", "", "Custom User-Agent string (overrides -ua-type)")
	uaType  := flag.String("ua-type", "random", "UA browser family: chrome, firefox, safari, random")
	tldFile := flag.String("tld-list", "", "Path to custom TLD wordlist (one per line, overrides built-in list)")
	flag.Parse()

	// Load custom TLD list if provided
	if *tldFile != "" {
		loaded, err := loadTLDs(*tldFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[!] Cannot load TLD list: %v\n", err)
			os.Exit(1)
		}
		if len(loaded) == 0 {
			fmt.Fprintf(os.Stderr, "[!] TLD list is empty: %s\n", *tldFile)
			os.Exit(1)
		}
		tlds = loaded
		fmt.Fprintf(os.Stderr, "[+] Loaded %d TLDs from %s\n", len(tlds), *tldFile)
	}

	cfg := &Config{
		depth:     *depth,
		threads:   *threads,
		outputFile: *output,
		silent:    *silent,
		timeout:   time.Duration(*timeoutSec) * time.Second,
		statusRaw: *statusCodes,
		filter:    parseStatusCodes(*statusCodes),
		userAgent: *ua,
		uaType:    *uaType,
	}

	var inputs []string

	// From -l flag
	if *inputList != "" {
		f, err := os.Open(*inputList)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[!] Cannot open input file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				inputs = append(inputs, line)
			}
		}
	}

	// From positional args
	for _, arg := range flag.Args() {
		inputs = append(inputs, strings.TrimSpace(arg))
	}

	// From stdin (pipe support)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				inputs = append(inputs, line)
			}
		}
	}

	if len(inputs) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: tldx [flags] <domain>\n")
		fmt.Fprintf(os.Stderr, "       tldx -l domains.txt\n")
		fmt.Fprintf(os.Stderr, "       echo example.com | tldx\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Deduplicate inputs
	seen := make(map[string]bool)
	var dedupedInputs []string
	for _, d := range inputs {
		if !seen[d] {
			seen[d] = true
			dedupedInputs = append(dedupedInputs, d)
		}
	}

	engine := NewEngine(cfg)
	defer engine.Close()
	engine.Run(dedupedInputs)
}