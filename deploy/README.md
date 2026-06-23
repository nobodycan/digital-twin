# digital-twin Deployment Runbook

This deployment package is a local production-shaped profile. It packages the Go server, web assets, config file, readiness endpoint, metrics endpoint, and local file-backed admin data. It does not add an external database, Kubernetes, OAuth, RBAC, or a managed provider dependency.

## Start Locally

```powershell
docker compose -f .\deploy\docker-compose.yml up --build
```

Then verify:

```powershell
Invoke-RestMethod http://localhost:8080/health
Invoke-RestMethod http://localhost:8080/ready
Invoke-WebRequest http://localhost:8080/metrics
go run .\cmd\smoke -base-url http://localhost:8080 -api-key change-me-local-api-key
```

Protected runtime and admin write routes require `Authorization: Bearer <DIGITAL_TWIN_SERVER_API_KEY>` when the API key is set.

## Data

Admin persona, memory, knowledge, tool policy, and audit records are stored under `/data/admin` in the container and mounted to the `digital_twin_admin_data` named volume.

Backup:

```powershell
docker run --rm -v digital-twin_digital_twin_admin_data:/data -v ${PWD}:/backup alpine tar czf /backup/digital-twin-admin-data.tgz -C /data .
```

Restore:

```powershell
docker run --rm -v digital-twin_digital_twin_admin_data:/data -v ${PWD}:/backup alpine sh -c "cd /data && tar xzf /backup/digital-twin-admin-data.tgz"
```

## Provider Profile

The default compose profile uses local/mock TTS and ASR. To test a real HTTP TTS provider, set these values through a private env file:

```text
DIGITAL_TWIN_TTS_PROVIDER=http
DIGITAL_TWIN_TTS_BASE_URL=https://provider.example/synthesize
DIGITAL_TWIN_TTS_API_KEY=...
```

Do not commit real provider keys. The runtime redacts configured secrets from startup summaries and readiness details.

## Fallback Without Docker

If Docker is unavailable, run the server directly:

```powershell
go test ./...
go run .\cmd\server -config .\configs\app.yaml
```

Use `.\scripts\verify_deploy.ps1` for static deployment-file checks.
