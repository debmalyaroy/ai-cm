<#
.SYNOPSIS
End-to-end integration test runner for ai-cm on Windows.
#>

Write-Host "======================================"
Write-Host " Starting Integration Test Suite...   "
Write-Host "======================================"

# Determine if a local database needs to be spun up via docker
# Checking if postgres port is open locally as pg_isready might not be in PATH
$portCheck = Test-NetConnection -ComputerName localhost -Port 5432 -InformationLevel Quiet
if (-not $portCheck) {
    Write-Host "Warning: No local postgres instance detected on port 5432." -ForegroundColor Yellow
    Write-Host "Please ensure you have a valid DATABASE_URL exported, or run 'docker-compose up -d db' to start the test database."
}
else {
    Write-Host "Local postgres instance detected."
}

# Run the integration suite locally
Write-Host "`nRunning E2E verification test suite..."

Set-Location -Path src\backend

# By explicitly calling out tests/e2e_test.go or a specific package, 
# we isolate the E2E verification from unit tests
go test ./tests/... -v -count=1 -timeout 120s

if ($LASTEXITCODE -eq 0) {
    Write-Host "======================================" -ForegroundColor Green
    Write-Host " E2E Integration Suite Passed!        " -ForegroundColor Green
    Write-Host "======================================" -ForegroundColor Green
}
else {
    Write-Host "======================================" -ForegroundColor Red
    Write-Host " E2E Integration Suite Failed.        " -ForegroundColor Red
    Write-Host "======================================" -ForegroundColor Red
    exit $LASTEXITCODE
}
