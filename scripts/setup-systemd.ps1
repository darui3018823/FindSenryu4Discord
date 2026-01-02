# 🛠️ FindSenryu4Discord Setup Script (PowerShell for Linux)
# 初回/再構築: ビルド + 配置 + systemd unit 作成/更新 + 起動

[CmdletBinding()]
param(
    [string]$ServiceName = 'findsenryu',
    [string]$User = 'root',
    [string]$Group = 'root',
    [string]$DeployDir = '/root/projects/FindSenryu4Discord',
    [string]$BinaryName = 'findsenryu',
    [switch]$Force
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Step { param([string]$m) Write-Host "[STEP] $m" }
function Info { param([string]$m) Write-Host "ℹ️  $m" }
function Ok { param([string]$m) Write-Host "✅ $m" }
function Warn { param([string]$m) Write-Host "⚠️  $m" }
function Err { param([string]$m) Write-Host "❌ $m" }

if(-not $IsLinux){ Err 'Linux (systemd) 専用'; exit 1 }
if(-not (Get-Command systemctl -ErrorAction SilentlyContinue)){ Err 'systemctl がありません'; exit 1 }
if(-not (Get-Command go -ErrorAction SilentlyContinue)){ Err 'go コマンドがありません'; exit 1 }

$RepoRoot = $DeployDir
Push-Location $RepoRoot
try {
    Write-Host "🛠️  Setup 開始 (service=$ServiceName deploy=$DeployDir)"

    # 既存停止
    if(& systemctl list-units --type=service --all | Select-String -Quiet "^$ServiceName.service"){
        Step '既存サービス停止'
        sudo systemctl stop $ServiceName || $true
        
        # 既存のサービスを無効化して完全に削除（再作成のため）
        sudo systemctl disable $ServiceName || $true
        sudo rm "/etc/systemd/system/$ServiceName.service" || $true
        sudo systemctl daemon-reload
        Info '既存サービスを完全に削除しました（再作成のため）'
    } else { Info '既存サービスなし' }

    # デプロイ先ディレクトリ確認
    if(-not (Test-Path $DeployDir)){
        Err "DeployDir が存在しません: $DeployDir"
        exit 1
    }
    Info "DeployDir 確認完了: $DeployDir"

    # .env 生成 (存在しない場合)
    $EnvFile = Join-Path $DeployDir '.env'
    if(-not (Test-Path $EnvFile) -or $Force){
        Step '.env 生成'
        $token = Read-Host "Discord Bot Token を入力してください"
        $playing = Read-Host "Playing ステータス (Optional, Enter for empty)"
        $clientId = Read-Host "Client ID (Optional, Enter for empty)"
        
@"
DISCORD_TOKEN=$token
DISCORD_PLAYING=$playing
DISCORD_CLIENT_ID=$clientId
"@ | sudo tee $EnvFile > $null
        sudo chown "${User}:${Group}" $EnvFile
        sudo chmod 600 $EnvFile
        Ok ".env を生成しました"
    } else { 
        Info '.env 既存 ( --Force で再生成 )'
    }

    # ビルド
    $BinaryPath = Join-Path $DeployDir $BinaryName
    Step 'ビルド'
    $Tmp = "$BinaryPath.new"
    bash -c "go build -o '$Tmp' -ldflags '-s -w' ." 2>&1 | ForEach-Object { $_ }
    if(-not (Test-Path $Tmp)){ throw 'ビルド成果物なし' }
    sudo mv $Tmp $BinaryPath
    sudo chmod +x $BinaryPath
    Ok "バイナリ配置: $BinaryPath"

    # データディレクトリ作成
    Step 'データディレクトリ準備'
    if(-not (Test-Path "$DeployDir/data")){ sudo mkdir -p "$DeployDir/data" }
    sudo chown -R "${User}:${Group}" "$DeployDir/data"
    sudo chmod -R 755 "$DeployDir/data"
    Ok "データディレクトリ準備完了"

    # systemd unit
    Step 'systemd unit 作成/更新'
    $UnitPath = "/etc/systemd/system/$ServiceName.service"
    $Unit = @"
[Unit]
Description=FindSenryu4Discord Bot
After=network.target
Wants=network.target

[Service]
Type=simple
User=$User
Group=$Group
WorkingDirectory=$DeployDir
ExecStart=$DeployDir/$BinaryName
Restart=on-failure
RestartSec=10
LimitNOFILE=65535
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=false
ReadWritePaths=$DeployDir/data

[Install]
WantedBy=multi-user.target
"@
    $Unit | sudo tee $UnitPath > $null
    
    Step 'systemd reload & enable & start'
    sudo systemctl daemon-reload
    sudo systemctl enable $ServiceName
    sudo systemctl start $ServiceName
    $state = (& systemctl is-active $ServiceName 2>$null).Trim()
    Ok "サービス起動 state=$state"
    
    Write-Host ""
    Write-Host "📋 確認コマンド:"
    Write-Host "  ステータス: sudo systemctl status $ServiceName"
    Write-Host "  ログ:       sudo journalctl -u $ServiceName -f"
    Write-Host "  再起動:     sudo systemctl restart $ServiceName"
    Write-Host "  停止:       sudo systemctl stop $ServiceName"
    Write-Host ""
    Write-Host '🎉 Setup 完了'
}
catch {
    Err "エラー: $($_.Exception.Message)"
    exit 1
}
finally { Pop-Location }
