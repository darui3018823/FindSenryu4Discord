# 🔄 FindSenryu4Discord Rebuild Script (PowerShell for Linux)

[CmdletBinding()]
param(
    [string]$ServiceName = 'findsenryu',
    [string]$BinaryName = 'findsenryu',
    [string]$DeployDir = '/root/projects/FindSenryu4Discord',
    [string]$Branch = 'master',
    [switch]$SkipGit,
    [switch]$SkipDeps,
    [switch]$SkipBuild,
    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Write-Step { param([string]$Message) Write-Host "[STEP] $Message" }
function Write-Info { param([string]$Message) Write-Host "ℹ️  $Message" }
function Write-Success { param([string]$Message) Write-Host "✅ $Message" }
function Write-Warn { param([string]$Message) Write-Host "⚠️  $Message" }
function Write-Err { param([string]$Message) Write-Host "❌ $Message" }

function Assert-LastExit($Label) { if ($LASTEXITCODE -ne 0) { throw "'$Label' 失敗 (exit=$LASTEXITCODE)" } }

function Test-SystemdEnabled {
    param([string]$Name)
    & systemctl is-enabled $Name *> $null
    return ($LASTEXITCODE -eq 0)
}

function Get-SystemdActiveState {
    param([string]$Name)
    try { (& systemctl is-active $Name 2>$null).Trim() } catch { 'unknown' }
}

Write-Host "🔄 FindSenryu4Discord リビルド開始 (service=$ServiceName)"

if(-not $IsLinux){ Write-Err 'Linux (systemd) 専用スクリプト'; exit 1 }
if(-not (Get-Command systemctl -ErrorAction SilentlyContinue)){ Write-Err 'systemctl が見つかりません'; exit 1 }
if(-not (Get-Command go -ErrorAction SilentlyContinue)){ Write-Err 'go コマンドが見つかりません'; exit 1 }
if(-not (Test-Path $DeployDir)){ Write-Err "DeployDir $DeployDir が存在しません。初回は ./scripts/setup-systemd.ps1"; exit 1 }

Push-Location $DeployDir
try {
    # 停止
    if(Test-SystemdEnabled $ServiceName){
        Write-Step 'systemd サービス停止'
        if(-not $DryRun){ sudo systemctl stop $ServiceName; Assert-LastExit "systemctl stop $ServiceName" }
    } else { 
        Write-Warn "systemdサービス($ServiceName) は未有効化" 
    }

    # Git 更新
    if(-not $SkipGit -and (Test-Path .git)){
        Write-Step "Git 更新 (branch=$Branch)"
        if(-not $DryRun){ 
            git fetch origin $Branch; Assert-LastExit 'git fetch'
            git checkout $Branch
            git pull origin $Branch; Assert-LastExit 'git pull' 
        }
    } else {
        Write-Warn 'Git 更新をスキップ (--SkipGit または .git 無し)' 
    }

    # 依存 (Go modules)
    if(-not $SkipDeps){
        Write-Step 'Go Modules 同期'
        if(-not $DryRun){ 
            go mod tidy; Assert-LastExit 'go mod tidy'
            go mod download; Assert-LastExit 'go mod download' 
        }
    } else {
        Write-Warn '依存同期をスキップ (--SkipDeps)' 
    }

    $BinaryPath = Join-Path $DeployDir $BinaryName
    $Tmp = "$BinaryPath.new"

    if (Test-Path $Tmp) { Remove-Item -Force $Tmp }

    # ビルド
    if(-not $SkipBuild){
        Write-Step 'ビルド'
        $cmd = "go build -o '$Tmp' -ldflags '-s -w' ."
        Write-Info $cmd
        if(-not $DryRun){ 
            bash -c $cmd 2>&1 | ForEach-Object { $_ }
            Assert-LastExit 'go build'
            if(-not (Test-Path $Tmp -PathType Leaf)){ throw 'ビルド成果物が見つかりません' }
        }
    } else {
        Write-Warn 'ビルドをスキップ (--SkipBuild)' 
    }

    # データディレクトリ確認
    Write-Step 'データディレクトリ確認'
    if(-not $DryRun){
        if(-not (Test-Path "$DeployDir/data")){ sudo mkdir -p "$DeployDir/data" }
        sudo chmod -R 755 "$DeployDir/data"
    }

    # バイナリ差替
    if(-not $SkipBuild -and -not $DryRun){
        Write-Step 'バイナリ差替'
        sudo mv $Tmp $BinaryPath
        sudo chmod +x $BinaryPath
    }

    # 起動
    if(Test-SystemdEnabled $ServiceName){
        Write-Step 'systemd サービス起動'
        if(-not $DryRun){ sudo systemctl start $ServiceName; Assert-LastExit "systemctl start $ServiceName" }
        $state = Get-SystemdActiveState $ServiceName
        Write-Success "再起動完了 state=$state"
    } else {
        Write-Err 'systemdサービスが有効化されていません。./scripts/setup-systemd.ps1 を実行してください'
        exit 1
    }

    Write-Host ''
    Write-Host '🎉 リビルド完了'
    Write-Host "📝 Journal: sudo journalctl -u $ServiceName -f"
    exit 0
} catch {
    Write-Err "エラー: $($_.Exception.Message)"
    exit 1
} finally {
    Pop-Location
}
