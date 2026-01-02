param(
    [string]$RepoRoot = (Resolve-Path ".").Path,
    [string]$ServiceUser = "findsenryu",
    [string]$InstallDir = "/opt/findsenryu",
    [string]$ServiceName = "findsenryu.service"
)

if (-not $IsLinux) {
    Write-Error "This script must run on Linux with systemd." -ErrorAction Stop
}

# Check for administrative privileges
if (-not ([Security.Principal.WindowsPrincipal]([Security.Principal.WindowsIdentity]::GetCurrent())).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Error "Run as root (sudo) so users, directories, and service units can be created." -ErrorAction Stop
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go toolchain is required but was not found in PATH." -ErrorAction Stop
}

$servicePath = "/etc/systemd/system/$ServiceName"

# Ensure service user exists
$null = & id -u $ServiceUser 2>$null
if ($LASTEXITCODE -ne 0) {
    & useradd --system --home $InstallDir --shell /usr/sbin/nologin $ServiceUser
}

# Prepare install directory
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# Build binary into install directory
Push-Location $RepoRoot
try {
    & go build -o "$InstallDir/findsenryu"
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

# Place config.toml if missing
if (-not (Test-Path "$InstallDir/config.toml")) {
    if (Test-Path (Join-Path $RepoRoot "config.toml")) {
        Copy-Item (Join-Path $RepoRoot "config.toml") "$InstallDir/config.toml"
    } elseif (Test-Path (Join-Path $RepoRoot "sample.config.toml")) {
        Copy-Item (Join-Path $RepoRoot "sample.config.toml") "$InstallDir/config.toml"
    } else {
        Write-Error "No config.toml or sample.config.toml found in repo. Create one before running." -ErrorAction Stop
    }
}

# Ensure data directory for SQLite/Ledis
if (-not (Test-Path (Join-Path $InstallDir "data"))) {
    New-Item -ItemType Directory -Path (Join-Path $InstallDir "data") -Force | Out-Null
}

# Permissions for service user
& chown -R "${ServiceUser}:${ServiceUser}" $InstallDir

# Write systemd unit
$unitContent = @"
[Unit]
Description=FindSenryu4Discord bot (systemd)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$ServiceUser
Group=$ServiceUser
WorkingDirectory=$InstallDir
ExecStart=$InstallDir/findsenryu
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
Restart=on-failure
RestartSec=3
RuntimeDirectory=findsenryu

[Install]
WantedBy=multi-user.target
"@

Set-Content -Path $servicePath -Value $unitContent -Encoding ascii

& systemctl daemon-reload
& systemctl enable --now $ServiceName

Write-Host "Systemd unit installed and started: $ServiceName"
Write-Host "Edit $InstallDir/config.toml to set your Discord token and settings."
