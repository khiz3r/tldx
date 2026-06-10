# tldx

TLD permutation bruteforcer. Takes a domain, appends every known TLD, probes them all, returns hits.

```
echo "zoho" | ./tldx
./tldx -l domains.txt -o results.txt
./tldx zoho -ua-type chrome -sc 200-299,403
```

## Install

```bash
git clone https://github.com/khiz3r/tldx
cd tldx
go build -o tldx main.go
```

## Usage

```
tldx [flags] <domain>
tldx -l domains.txt
echo example | tldx
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-d` | `1` | Recursion depth (0 = unlimited) |
| `-t` | `50` | Concurrent threads |
| `-sc` | `200-299,301,302,403,401` | Status codes to treat as hits |
| `-timeout` | `10` | HTTP timeout in seconds |
| `-ua-type` | `random` | UA browser family: `chrome`, `firefox`, `safari`, `random` |
| `-ua` | | Custom User-Agent string (overrides `-ua-type`) |
| `-l` | | File with domains, one per line |
| `-o` | | Output file path |
| `-silent` | | Only print hits, no banner |

## Wildcard expansion

Trailing `.*` expands across all TLDs before probing:

```
echo "social.zoho.*" | ./tldx
```

Probes `social.zoho.com`, `social.zoho.co.uk`, `social.zoho.com.au` ... (271 candidates).

## How depth works

Each hit at depth N becomes a base for depth N+1:

```
depth 1: zoho → zoho.com, zoho.net, zoho.co.uk ...
depth 2: zoho.com → zoho.com.au, zoho.com.br ...
```

Default `-d 1` probes once and stops. Use `-d 0` only if you know what you're doing.

## Covers

271 TLDs — country codes, generics, and second-level combos (`com.au`, `co.uk`, `com.br` etc).
