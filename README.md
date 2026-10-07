# Go Starter Service

A minimal, independent starter repository for Session 01 of the Software Engineering Lab.
The task is to run the same health endpoint on every team member's computer.
No database, authentication, or external service is required.

## Requirements

- Go 1.26.x. No external Go packages are required.
- Git to clone and push the repository.
- Internet access for the first dependency download.
- Docker is optional for running this service; use it only if you choose the Docker route below.

## API contract

| Item | Value |
| --- | --- |
| Method | `GET` |
| Path | `/health` |
| HTTP status | `200 OK` |
| Content type | `application/json` (an optional charset is allowed) |
| JSON body | `{"status":"ok"}` |
| Local URL | `http://127.0.0.1:8080/health` |

JSON whitespace is insignificant. Header order and framework-specific headers may differ
between languages. Error body formats are not part of the shared contract.
`POST /health` returns 405; an unknown route returns 404.
This endpoint checks that the application is responding, not the health of future dependencies.

## Run locally

Open a terminal in this repository's root, then run:

```bash
go version
go run .
```

Keep that terminal open. Stop the server with `Ctrl+C`.

Build without keeping a server running:

```bash
go build ./...
```

## Check the service

In a second terminal on Windows PowerShell:

```powershell
curl.exe -i http://127.0.0.1:8080/health
```

On macOS or Linux:

```bash
curl -i http://127.0.0.1:8080/health
```

Expect HTTP 200, JSON content type, and `{"status":"ok"}`.
You can also open the URL in your browser.

For an optional automated contract check, install Python 3.10+ and run:

```bash
python scripts/verify_health.py
```

On Windows, use `py scripts/verify_health.py` if `python` is not available;
on macOS/Linux use `python3`. No extra Python packages are needed for this check.
The server must already be running. The script checks GET, the rejected POST method,
and an unknown route. It exits with a nonzero status on failure.

## Run with Docker

From the repository root:

```bash
docker build -t starter-go .
docker run --rm -p 127.0.0.1:8080:8080 starter-go
```

The container listens on all container interfaces; the published port is restricted to
localhost on your machine. Stop the foreground container with `Ctrl+C`.
The Docker image is for a teaching starter; registry tags receive updates and are not digest-pinned.

## Port conflicts

Run only one starter on port 8080 at a time. Stop the previous server first.
To use port 8081 with Docker, change the mapping to `127.0.0.1:8081:8080`.
Then check `http://127.0.0.1:8081/health`.

Local port override (PowerShell):

```powershell
$env:LISTEN_ADDR = "127.0.0.1:8081"
go run .
```

On macOS/Linux: `LISTEN_ADDR=127.0.0.1:8081 go run .`.
The module path `example.com/softwarelab/starter` is a local teaching placeholder; no remote Go module is fetched for it.

## Files to know

- `main.go`: health endpoint implementation.
- `Dockerfile`: optional container build and startup.
- `.gitignore`: excludes generated files and local settings.
- `scripts/verify_health.py`: optional black-box API verification.

## Publish this starter as a new repository

Create an **empty** GitHub repository named `starter-go`. Do not initialize it with a README,
license, or gitignore. From this extracted folder (which has no Git history), run:

```bash
git init -b main
git add .
git commit -m "Add minimal health service starter"
git remote add origin https://github.com/YOUR_ORG/starter-go.git
git push -u origin main
```

Replace `YOUR_ORG` with your GitHub organization or account. Configure your own Git name
and email first. These commands are for a new repository, not an existing working copy.
After publishing, the repository can optionally be marked as a template in GitHub settings.

## Session 01 completion checklist

- [ ] Three team members and the chosen language are registered.
- [ ] The unchanged starter code has been pushed to the team's `main` branch.
- [ ] Every member can clone the team repository.
- [ ] Every member gets the successful `/health` response locally.
- [ ] The report includes the team repository URL and evidence of each member's successful run.

## Troubleshooting

- **Connection refused:** start the service and check its terminal output and port.
- **Address already in use:** stop the other service or use the override above.
- **404 on `/`:** expected; request `/health` instead.
- **Dependency download failed:** check network/proxy access to the package registry.
- **Command not found:** install the SDK/tool listed above and reopen the terminal.

## Official reference

[Go web service documentation](https://pkg.go.dev/net/http)
