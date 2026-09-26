# Install or update kos, the knowledge OS CLI, on Windows.
#
#   gh release download --repo rendis/knowledge-os --pattern install-kos.ps1 --output install-kos.ps1; ./install-kos.ps1
#
# Environment: KOS_REPO (default rendis/knowledge-os), KOS_VERSION (default latest),
# KOS_INSTALL_DIR (default $HOME\.local\bin), KOS_DOWNLOAD_URL (base URL of the assets).
$ErrorActionPreference = 'Stop'
$repo = if ($env:KOS_REPO) { $env:KOS_REPO } else { 'rendis/knowledge-os' }
$version = if ($env:KOS_VERSION) { $env:KOS_VERSION } else { 'latest' }
$dir = if ($env:KOS_INSTALL_DIR) { $env:KOS_INSTALL_DIR } else { Join-Path $HOME '.local\bin' }
$arch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$asset = if ($arch -eq 'ARM64') { 'kos-windows-arm64.exe' } else { 'kos-windows-amd64.exe' }
$work = Join-Path ([System.IO.Path]::GetTempPath()) ("kos-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $work | Out-Null
try {
  if ($env:KOS_DOWNLOAD_URL) {
    $base = $env:KOS_DOWNLOAD_URL.TrimEnd('/')
    Invoke-WebRequest "$base/$asset" -OutFile (Join-Path $work $asset)
    Invoke-WebRequest "$base/SHA256SUMS" -OutFile (Join-Path $work 'SHA256SUMS')
  } else {
    $tag = @()
    if ($version -ne 'latest') { $tag = @("v$version") }
    gh release download @tag --repo $repo --pattern $asset --pattern SHA256SUMS --dir $work
    if ($LASTEXITCODE -ne 0) { throw "gh could not read $repo: log in with an account that can read it" }
  }
  $expected = (Get-Content (Join-Path $work 'SHA256SUMS') | Where-Object { ($_ -split '\s+')[1] -eq $asset } | ForEach-Object { ($_ -split '\s+')[0] })
  $actual = (Get-FileHash (Join-Path $work $asset) -Algorithm SHA256).Hash.ToLower()
  if (-not $expected -or $expected -ne $actual) { throw "checksum mismatch for ${asset}: refused" }
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
  Copy-Item (Join-Path $work $asset) (Join-Path $dir 'kos.exe') -Force
  Write-Output "kos: installed at $(Join-Path $dir 'kos.exe')"
  if (-not ($env:PATH -split ';' | Where-Object { $_ -eq $dir })) { Write-Output "kos: add $dir to your PATH" }
} finally {
  Remove-Item -Recurse -Force $work
}
