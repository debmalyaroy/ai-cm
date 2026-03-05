<#
.SYNOPSIS
    AI-CM: AWS EC2 + RDS Start / Stop Helper (Windows PowerShell)
    Manages your EC2 instance and RDS database to stay within Free Tier limits.

.DESCRIPTION
    Prerequisites:
    - AWS CLI v2 installed: winget install Amazon.AWSCLI
    - Configured: aws configure  (or use IAM Identity Center)
    - EC2_INSTANCE_ID and RDS_INSTANCE_ID set in config/.env.prod or passed directly

.PARAMETER Action
    start | stop | status

.PARAMETER EC2InstanceId
    Override EC2 instance ID (or set EC2_INSTANCE_ID in config/.env.prod)

.PARAMETER RdsInstanceId
    Override RDS instance ID (or set RDS_INSTANCE_ID in config/.env.prod)

.PARAMETER Region
    AWS region (default: us-east-1)

.EXAMPLE
    .\scripts\aws_startstop.ps1 start
    .\scripts\aws_startstop.ps1 stop
    .\scripts\aws_startstop.ps1 status
#>

param(
    [Parameter(Mandatory=$true)]
    [ValidateSet("start", "stop", "status")]
    [string]$Action,

    [string]$EC2InstanceId = "",
    [string]$RdsInstanceId = "",
    [string]$Region = ""
)

$ErrorActionPreference = "Stop"

# ── Load IDs from config/.env.prod ───────────────────────────────────────────
$EnvFile = Join-Path $PSScriptRoot "..\config\.env.prod"
if (Test-Path $EnvFile) {
    Get-Content $EnvFile | ForEach-Object {
        if ($_ -match '^([A-Z_]+)=(.+)$') {
            $key = $Matches[1]; $val = $Matches[2].Trim()
            if ($key -eq "EC2_INSTANCE_ID" -and $EC2InstanceId -eq "") { $EC2InstanceId = $val }
            if ($key -eq "RDS_INSTANCE_ID" -and $RdsInstanceId -eq "") { $RdsInstanceId = $val }
            if ($key -eq "AWS_REGION"      -and $Region -eq "")        { $Region = $val }
        }
    }
}
if ($Region -eq "") { $Region = "us-east-1" }

if ($EC2InstanceId -eq "" -or $RdsInstanceId -eq "") {
    Write-Host "❌ EC2_INSTANCE_ID and RDS_INSTANCE_ID must be set." -ForegroundColor Red
    Write-Host "   Add them to config/.env.prod:"
    Write-Host "     EC2_INSTANCE_ID=i-0123456789abcdef0"
    Write-Host "     RDS_INSTANCE_ID=aicm-postgres"
    exit 1
}

function Get-EC2State {
    $state = aws ec2 describe-instances `
        --instance-ids $EC2InstanceId `
        --region $Region `
        --query "Reservations[0].Instances[0].State.Name" `
        --output text 2>$null
    return if ($state) { $state } else { "unknown" }
}

function Get-RDSState {
    $state = aws rds describe-db-instances `
        --db-instance-identifier $RdsInstanceId `
        --region $Region `
        --query "DBInstances[0].DBInstanceStatus" `
        --output text 2>$null
    return if ($state) { $state } else { "unknown" }
}

switch ($Action) {

# ── STATUS ────────────────────────────────────────────────────────────────────
"status" {
    $ec2State = Get-EC2State
    $rdsState = Get-RDSState
    Write-Host "──────────────────────────────────────────"
    Write-Host " EC2  ($EC2InstanceId): $ec2State"
    Write-Host " RDS  ($RdsInstanceId): $rdsState"
    Write-Host "──────────────────────────────────────────"
}

# ── START ─────────────────────────────────────────────────────────────────────
"start" {
    Write-Host "🟢 Starting RDS instance: $RdsInstanceId..." -ForegroundColor Green
    $rdsState = Get-RDSState
    if ($rdsState -eq "available") {
        Write-Host "   RDS is already running."
    } elseif ($rdsState -eq "stopped") {
        aws rds start-db-instance --db-instance-identifier $RdsInstanceId --region $Region | Out-Null
        Write-Host "   Waiting for RDS to become available (may take 2-4 min)..."
        aws rds wait db-instance-available --db-instance-identifier $RdsInstanceId --region $Region
        Write-Host "   ✓ RDS is available." -ForegroundColor Green
    } else {
        Write-Host "   RDS state is '$rdsState' — cannot start. Check AWS Console." -ForegroundColor Yellow
    }

    Write-Host "🟢 Starting EC2 instance: $EC2InstanceId..." -ForegroundColor Green
    $ec2State = Get-EC2State
    if ($ec2State -eq "running") {
        Write-Host "   EC2 is already running."
    } elseif ($ec2State -eq "stopped") {
        aws ec2 start-instances --instance-ids $EC2InstanceId --region $Region | Out-Null
        Write-Host "   Waiting for EC2 to reach running state..."
        aws ec2 wait instance-running --instance-ids $EC2InstanceId --region $Region
        Write-Host "   ✓ EC2 is running." -ForegroundColor Green
    } else {
        Write-Host "   EC2 state is '$ec2State' — cannot start. Check AWS Console." -ForegroundColor Yellow
    }

    $publicIP = aws ec2 describe-instances `
        --instance-ids $EC2InstanceId --region $Region `
        --query "Reservations[0].Instances[0].PublicIpAddress" `
        --output text 2>$null

    Write-Host ""
    Write-Host "✅ Services started." -ForegroundColor Green
    Write-Host "   App URL: http://$publicIP"
    Write-Host "   SSH:     ssh -i your-key.pem ec2-user@$publicIP"
    Write-Host ""
    Write-Host "⚠️  Stop services when not in use to stay within Free Tier:" -ForegroundColor Yellow
    Write-Host "   .\scripts\aws_startstop.ps1 stop"
}

# ── STOP ──────────────────────────────────────────────────────────────────────
"stop" {
    Write-Host "🔴 Stopping EC2 instance: $EC2InstanceId..." -ForegroundColor Red
    $ec2State = Get-EC2State
    if ($ec2State -eq "stopped") {
        Write-Host "   EC2 is already stopped."
    } elseif ($ec2State -eq "running") {
        aws ec2 stop-instances --instance-ids $EC2InstanceId --region $Region | Out-Null
        Write-Host "   Waiting for EC2 to stop..."
        aws ec2 wait instance-stopped --instance-ids $EC2InstanceId --region $Region
        Write-Host "   ✓ EC2 stopped." -ForegroundColor Green
    } else {
        Write-Host "   EC2 state is '$ec2State' — may not be stoppable right now." -ForegroundColor Yellow
    }

    Write-Host "🔴 Stopping RDS instance: $RdsInstanceId..." -ForegroundColor Red
    $rdsState = Get-RDSState
    if ($rdsState -eq "stopped") {
        Write-Host "   RDS is already stopped."
    } elseif ($rdsState -eq "available") {
        aws rds stop-db-instance --db-instance-identifier $RdsInstanceId --region $Region | Out-Null
        Write-Host "   RDS stop initiated (no need to wait)."
        Write-Host "   ✓ RDS stopping." -ForegroundColor Green
    } else {
        Write-Host "   RDS state is '$rdsState' — cannot stop right now." -ForegroundColor Yellow
    }

    Write-Host ""
    Write-Host "✅ Stop commands issued." -ForegroundColor Green
    Write-Host "   EC2 and RDS are not billed while stopped."
    Write-Host "   Note: AWS auto-starts RDS after 7 days (AWS limitation)." -ForegroundColor Yellow
}

}
