# setup-windows.ps1
# Запустити від імені Адміністратора:
#   Right-click → "Run as Administrator"
# або у PowerShell:
#   Set-ExecutionPolicy -Scope Process Bypass; .\setup-windows.ps1

$PROXY_HOST = "100.66.97.93"   # Oracle server Tailscale IP
$HTTP_PORT  = "8888"
$SOCKS_PORT = "1080"
$PROXY_HTTP  = "http://$PROXY_HOST`:$HTTP_PORT"
$PROXY_SOCKS = "socks5://$PROXY_HOST`:$SOCKS_PORT"

Write-Host "`n=== Swiss Geo-Masking: Windows Setup ===" -ForegroundColor Cyan

# ── 1. Timezone ───────────────────────────────────────────────────────────────
Write-Host "`n[1] Setting timezone to W. Europe Standard Time (Zurich)..."
try {
    Set-TimeZone -Id "W. Europe Standard Time"
    Write-Host "    Timezone: OK" -ForegroundColor Green
} catch {
    Write-Host "    FAILED: run as Administrator" -ForegroundColor Red
}

# ── 2. System-wide HTTPS proxy (Windows Internet Settings) ───────────────────
Write-Host "`n[2] Setting system HTTP proxy ($PROXY_HOST`:$HTTP_PORT)..."
try {
    $regPath = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings"
    Set-ItemProperty -Path $regPath -Name ProxyEnable    -Value 1
    Set-ItemProperty -Path $regPath -Name ProxyServer    -Value "$PROXY_HOST`:$HTTP_PORT"
    Set-ItemProperty -Path $regPath -Name ProxyOverride  -Value "localhost;127.*;10.*;172.16.*;192.168.*;100.66.97.93;<local>"
    Write-Host "    System proxy: OK" -ForegroundColor Green
} catch {
    Write-Host "    FAILED: $($_.Exception.Message)" -ForegroundColor Red
}

# ── 3. User environment variables (for GoLaw, Python, Node.js apps) ──────────
Write-Host "`n[3] Setting user environment variables..."
[System.Environment]::SetEnvironmentVariable("HTTP_PROXY",  $PROXY_HTTP,  "User")
[System.Environment]::SetEnvironmentVariable("HTTPS_PROXY", $PROXY_HTTP,  "User")
[System.Environment]::SetEnvironmentVariable("http_proxy",  $PROXY_HTTP,  "User")
[System.Environment]::SetEnvironmentVariable("https_proxy", $PROXY_HTTP,  "User")
[System.Environment]::SetEnvironmentVariable("SOCKS5_PROXY",$PROXY_SOCKS, "User")
[System.Environment]::SetEnvironmentVariable("NO_PROXY",    "localhost,127.0.0.1,::1,100.66.97.93", "User")
[System.Environment]::SetEnvironmentVariable("no_proxy",    "localhost,127.0.0.1,::1,100.66.97.93", "User")
Write-Host "    Env vars: OK" -ForegroundColor Green

# ── 4. Chrome shortcut with Swiss proxy + WebRTC disabled ────────────────────
Write-Host "`n[4] Creating Chrome shortcut with Swiss proxy..."
$chromePaths = @(
    "$env:LOCALAPPDATA\Google\Chrome\Application\chrome.exe",
    "$env:PROGRAMFILES\Google\Chrome\Application\chrome.exe",
    "${env:PROGRAMFILES(x86)}\Google\Chrome\Application\chrome.exe"
)
$chromeExe = $chromePaths | Where-Object { Test-Path $_ } | Select-Object -First 1

if ($chromeExe) {
    $chromeArgs = @(
        "--proxy-server=socks5://$PROXY_HOST`:$SOCKS_PORT",
        "--proxy-bypass-list=localhost;127.*;100.66.97.93",
        "--enforce-webrtc-ip-permission-check",
        "--webrtc-ip-handling-policy=disable_non_proxied_udp",
        "--lang=de-CH",
        "--accept-lang=de-CH,de,en",
        "--user-data-dir=`"$env:LOCALAPPDATA\Google\Chrome\SwissProfile`""
    ) -join " "

    $WshShell = New-Object -ComObject WScript.Shell
    $desktop  = [System.Environment]::GetFolderPath("Desktop")
    $lnk      = $WshShell.CreateShortcut("$desktop\Chrome Swiss.lnk")
    $lnk.TargetPath       = $chromeExe
    $lnk.Arguments        = $chromeArgs
    $lnk.Description      = "Chrome via Swiss proxy (CH exit)"
    $lnk.WorkingDirectory = Split-Path $chromeExe
    $lnk.Save()
    Write-Host "    Shortcut created: Desktop\Chrome Swiss.lnk" -ForegroundColor Green
} else {
    Write-Host "    Chrome not found - skipped" -ForegroundColor Yellow
}

# ── 5. Comet / Perplexity — dedicated shortcut with explicit proxy flags ──────
Write-Host "`n[5] Creating Comet/Perplexity shortcut with Swiss proxy..."
$cometExe = "$env:LOCALAPPDATA\Perplexity\Comet\Application\comet.exe"

if (Test-Path $cometExe) {
    $cometArgs = @(
        "--proxy-server=socks5://$PROXY_HOST`:$SOCKS_PORT",
        "--proxy-bypass-list=localhost;127.*;100.66.97.93",
        "--enforce-webrtc-ip-permission-check",
        "--webrtc-ip-handling-policy=disable_non_proxied_udp",
        "--lang=de-CH",
        "--accept-lang=de-CH,de,en"
    ) -join " "

    $WshShell2 = New-Object -ComObject WScript.Shell
    $desktop2  = [System.Environment]::GetFolderPath("Desktop")
    $lnk2      = $WshShell2.CreateShortcut("$desktop2\Comet Swiss.lnk")
    $lnk2.TargetPath       = $cometExe
    $lnk2.Arguments        = $cometArgs
    $lnk2.Description      = "Comet (Perplexity) via Swiss proxy (CH exit)"
    $lnk2.WorkingDirectory = Split-Path $cometExe
    $lnk2.Save()
    Write-Host "    Shortcut created: Desktop\Comet Swiss.lnk" -ForegroundColor Green

    # Pin to taskbar hint
    Write-Host "    Use 'Comet Swiss' shortcut - NOT the default Comet icon" -ForegroundColor Yellow
} else {
    Write-Host "    Comet not found at $cometExe - skipped" -ForegroundColor Yellow
}

# ── 6. Verify ─────────────────────────────────────────────────────────────────
Write-Host "`n[6] Verification..."
Write-Host "    Testing proxy connectivity..."
try {
    $r = Invoke-WebRequest -Uri "https://ipinfo.io/json" `
         -Proxy $PROXY_HTTP -UseBasicParsing -TimeoutSec 15
    $j = $r.Content | ConvertFrom-Json
    Write-Host "    Exit IP  : $($j.ip)" -ForegroundColor Green
    Write-Host "    City     : $($j.city)" -ForegroundColor Green
    Write-Host "    Country  : $($j.country)" -ForegroundColor Green
    if ($j.country -eq "CH") {
        Write-Host "    RESULT   : Switzerland confirmed" -ForegroundColor Green
    } else {
        Write-Host "    WARNING  : not Switzerland! Check Tailscale" -ForegroundColor Red
    }
} catch {
    Write-Host "    FAILED: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "    Make sure Tailscale is running and you can reach $PROXY_HOST" -ForegroundColor Yellow
}

Write-Host "`n=== Done ===" -ForegroundColor Cyan
Write-Host "ВАЖЛИВО: Відкривай Chrome через ярлик 'Chrome Swiss' на робочому столі"
Write-Host "WebRTC заблоковано у цьому профілі. IP виходу: Zurich, CH."
Write-Host "Для GoLaw / Python агентів - environment variables вже встановлені.`n"
