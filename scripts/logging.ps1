# Shared logging helpers for scenario env-up scripts.
# Dot-source this file after computing $repoRoot:
#
#   $repoRoot = (Resolve-Path (Join-Path $PSScriptRoot ".." ".." "..")).Path
#   . (Join-Path $repoRoot "scripts" "logging.ps1")
#
# Add [CmdletBinding()] + param() to the calling script so -Verbose is supported.
# When -Verbose is passed, $VerbosePreference becomes 'Continue' and Write-Detail
# messages are shown.

function Write-Step {
    param([string]$Message)
    Write-Host "`n▶  $Message" -ForegroundColor Cyan
}

function Write-Info {
    param([string]$Message)
    Write-Host "   $Message" -ForegroundColor White
}

# Shown only when -Verbose is active (checks inherited $VerbosePreference).
function Write-Detail {
    param([string]$Message)
    if ($VerbosePreference -ne 'SilentlyContinue') {
        Write-Host "   [debug] $Message" -ForegroundColor DarkGray
    }
}

function Write-Ok {
    param([string]$Message)
    Write-Host "   ✅ $Message" -ForegroundColor Green
}

function Write-Warn {
    param([string]$Message)
    Write-Host "   ⚠  $Message" -ForegroundColor Yellow
}

# Dumps the last $Tail lines of a container's logs.
# Suppresses errors locally so a missing container does not mask the original failure.
function Show-ContainerLogs {
    param(
        [string]$ContainerName,
        [int]$Tail = 40
    )
    # Local scope: prevent a non-zero exit code from podman from throwing when
    # $PSNativeCommandUseErrorActionPreference = $true in the caller.
    $ErrorActionPreference = 'SilentlyContinue'
    $PSNativeCommandUseErrorActionPreference = $false

    $output = podman logs --tail $Tail $ContainerName 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Detail "container '$ContainerName' not accessible (exit code $LASTEXITCODE)"
        return
    }
    Write-Host "`n   ── logs: $ContainerName (last $Tail lines) ──" -ForegroundColor DarkYellow
    if ($output) {
        $output | ForEach-Object { Write-Host "      $_" -ForegroundColor DarkGray }
    }
    else {
        Write-Host "      (no output)" -ForegroundColor DarkGray
    }
    Write-Host "   ── end logs ──`n" -ForegroundColor DarkYellow
}

# Shows the status of all local containers (podman ps -a).
# Suppresses errors locally so this helper does not mask the original failure.
function Show-PodmanStatus {
    $ErrorActionPreference = 'SilentlyContinue'
    $PSNativeCommandUseErrorActionPreference = $false

    Write-Host "`n   ── container status (podman ps -a) ──" -ForegroundColor DarkYellow
    podman ps -a --format "table {{.Names}}\t{{.Status}}\t{{.Image}}" 2>&1 |
        ForEach-Object { Write-Host "      $_" -ForegroundColor DarkGray }
    Write-Host "   ── end status ──`n" -ForegroundColor DarkYellow
}
