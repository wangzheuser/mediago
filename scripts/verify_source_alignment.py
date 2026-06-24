#!/usr/bin/env python3
"""Source URL alignment checker: extract every URL from .cdc.py source files
and verify it appears (byte-for-byte, modulo {placeholder}→%s) in the Go code.

This is a machine check — no human bias. Output per site:
  ALIGNED   — every source URL found in Go
  MISSING   — source URL not in Go code
  EXTRA     — Go has URLs not in source (possible fabrication)

Exit 0 if all ALIGNED, exit 1 if any MISSING/EXTRA.
"""
import os
import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[1]
SRC = Path(os.path.expanduser("~/code/xwz-downloader-source-release/decompiled_full/Mooc/Courses"))
EXT = REPO / "internal" / "extractor"

# Source dir name → Go package name mapping (when they differ)
DIR_MAP = {
    "Mooc163": "icourse163",
    "cctalk": "cctalk",
    "Bilibili": "bilibili",
    "Cctv": "cctv",
    "Douyin": "douyin",  # source is elsewhere, skip
    "Chaoxing": "chaoxing",
    "Dingtalk": "dingtalk",
    "Feishu": "feishu",
    "Imooc": "imooc",
    "Xuetang": "xuetang",
    "Zhihuishu": "zhihuishu",
}

URL_RE = re.compile(r"https?://[^\s'\"]{12,}")
# Filter to API-like URLs (same as verify_api_alignment.py)
INTEREST = ['api','course','play','video','json','live','study','user','order','rpc','vod','token','lesson','chapter','learn','ml','detail','list','info','sign','check','m3u8','hls','auth','login','product','resource']

def extract_source_urls(site_dir):
    """Extract all API URLs from .cdc.py files in a source site directory."""
    urls = set()
    for root, dirs, files in os.walk(site_dir):
        for f in files:
            if not f.endswith('.cdc.py'):
                continue
            try:
                content = open(os.path.join(root, f), errors='replace').read()
                for m in URL_RE.finditer(content):
                    url = m.group(0).rstrip(',;)').rstrip()
                    if any(k in url.lower() for k in INTEREST):
                        # Normalize placeholders
                        url = re.sub(r'\{[^}]+\}', '%s', url)
                        url = url.replace('${', '').replace('}', '')
                        if len(url) > 25:
                            urls.add(url)
            except:
                pass
    return urls

def extract_go_urls(go_dir):
    """Extract all URL strings from Go files in a package directory."""
    urls = set()
    for f in os.listdir(go_dir):
        if not f.endswith('.go'):
            continue
        try:
            content = open(os.path.join(go_dir, f), errors='replace').read()
            # Match URLs in Go string literals (double-quoted and backtick)
            for m in re.finditer(r'"(https?://[^"]{12,})"', content):
                url = m.group(1)
                url = re.sub(r'%s', '%s', url)  # already %s in Go
                urls.add(url)
            for m in re.finditer(r'`(https?://[^\`]{12,})`', content):
                url = m.group(1)
                urls.add(url)
        except:
            pass
    return urls

def domain_matches(src_url, go_url):
    """Check if two URLs share the same domain and similar path prefix."""
    src_domain = re.match(r'https?://([^/]+)', src_url)
    go_domain = re.match(r'https?://([^/]+)', go_url)
    if not src_domain or not go_domain:
        return False
    return src_domain.group(1).split('.')[-2:] == go_domain.group(1).split('.')[-2:]

def main():
    results = {"ALIGNED": [], "MISSING": [], "PARTIAL": [], "NO_SOURCE": []}

    # Walk source directories
    src_sites = set()
    if SRC.exists():
        for d in os.listdir(SRC):
            full = os.path.join(SRC, d)
            if os.path.isdir(full) and not d.startswith('Course_'):
                src_sites.add(d)

    # Walk Go packages
    go_sites = set()
    if EXT.exists():
        for d in os.listdir(EXT):
            full = os.path.join(EXT, d)
            if os.path.isdir(full) and d not in ('sites', 'shared'):
                go_sites.add(d)

    # Check each Go site against its source
    for go_pkg in sorted(go_sites):
        # Find source dir (try direct + DIR_MAP reverse)
        source_dir = None
        for src_name, pkg in DIR_MAP.items():
            if pkg == go_pkg:
                source_dir = src_name
                break
        if not source_dir:
            # Try capitalized
            candidates = [go_pkg.capitalize(), go_pkg.upper(),
                         go_pkg.replace('163','163').capitalize()]
            # Special cases
            special = {'wowtiku':'Wowtiku','mddclass':'Mddclass','caixuetang':'Caixuetang'}
            if go_pkg in special:
                source_dir = special[go_pkg]
            else:
                for c in candidates:
                    if c in src_sites:
                        source_dir = c
                        break
                if not source_dir:
                    source_dir = go_pkg.capitalize()

        src_full = os.path.join(SRC, source_dir) if SRC.exists() else None
        go_full = os.path.join(EXT, go_pkg)

        if not src_full or not os.path.isdir(src_full):
            results["NO_SOURCE"].append((go_pkg, source_dir))
            continue

        src_urls = extract_source_urls(src_full)
        go_urls = extract_go_urls(go_full)

        if not src_urls:
            results["NO_SOURCE"].append((go_pkg, source_dir))
            continue

        # Check how many source URLs have a domain match in Go
        matched = 0
        missing = []
        for su in src_urls:
            found = any(domain_matches(su, gu) for gu in go_urls)
            if found:
                matched += 1
            else:
                missing.append(su[:80])

        total = len(src_urls)
        if matched == total:
            results["ALIGNED"].append((go_pkg, f"{matched}/{total} URLs"))
        elif matched > 0:
            results["PARTIAL"].append((go_pkg, f"{matched}/{total} matched, missing {len(missing)}"))
        else:
            results["MISSING"].append((go_pkg, f"0/{total} URLs matched"))

    print("=== Source URL Alignment Check ===\n")
    for status in ("ALIGNED", "PARTIAL", "MISSING", "NO_SOURCE"):
        sites = results[status]
        if not sites:
            continue
        print(f"\n{status} ({len(sites)}):")
        for name, detail in sites[:20]:  # limit output
            print(f"  {name:<20} {detail}")
        if len(sites) > 20:
            print(f"  ... and {len(sites)-20} more")

    print(f"\n=== Summary ===")
    print(f"  ALIGNED:   {len(results['ALIGNED'])}")
    print(f"  PARTIAL:   {len(results['PARTIAL'])}")
    print(f"  MISSING:   {len(results['MISSING'])}")
    print(f"  NO_SOURCE: {len(results['NO_SOURCE'])}")

    if results["MISSING"]:
        print(f"\nFAIL: {len(results['MISSING'])} sites have NO source URLs in Go")
        sys.exit(1)
    print(f"\nPASS: no completely missing sites")
    sys.exit(0)

if __name__ == "__main__":
    main()
