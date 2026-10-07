"""Check a running service using only the Python standard library."""
import argparse
import json
import sys
import urllib.error
import urllib.request


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("base_url", nargs="?", default="http://127.0.0.1:8080")
    args = parser.parse_args()
    base = args.base_url.rstrip("/")
    # Ignore environment proxy settings for this local verification.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        with opener.open(base + "/health", timeout=5) as response:
            status = response.status
            media_type = response.headers.get_content_type()
            body = json.loads(response.read())
        if status != 200 or media_type != "application/json" or body != {"status": "ok"}:
            raise ValueError(f"Unexpected response: {status}, {media_type}, {body!r}")
        print('PASS: GET /health -> 200 application/json {"status":"ok"}')
        request = urllib.request.Request(base + "/health", method="POST", data=b"")
        try:
            with opener.open(request, timeout=5) as response:
                raise ValueError(f"POST unexpectedly returned {response.status}")
        except urllib.error.HTTPError as error:
            if error.code != 405:
                raise ValueError(f"POST expected 405, got {error.code}") from error
        print("PASS: POST /health -> 405")
        try:
            with opener.open(base + "/not-a-route", timeout=5) as response:
                raise ValueError(f"Unknown route unexpectedly returned {response.status}")
        except urllib.error.HTTPError as error:
            if error.code != 404:
                raise ValueError(f"Unknown route expected 404, got {error.code}") from error
        print("PASS: unknown route -> 404")
        return 0
    except (OSError, ValueError) as error:
        print(f"FAIL: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
