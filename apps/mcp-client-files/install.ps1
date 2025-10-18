param(
  [string]$Version = "latest",
  [string]$BinDir = "$env:ProgramFiles\taskly-mcp",
  [string]$BaseUrl = "https://cdn.taskly.iy0306.com/docs/mcp/clients"
)

$ErrorActionPreference = "Stop"
$App = "taskly-mcp"

$os = "windows"
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$asset = "$App-$os-$arch.exe"

if ($Version -eq "latest") {
  $latestJson = Invoke-RestMethod -UseBasicParsing "$BaseUrl/latest.json"
  $Version = $latestJson.tag
}

$tmp = New-Item -ItemType Directory -Path ([System.IO.Path]::GetTempPath()) -Name ("taskly-mcp-" + [System.Guid]::NewGuid().ToString()) -Force
try {
  Write-Host "Downloading $asset ($Version) ..."
  Invoke-WebRequest -UseBasicParsing -Uri "$BaseUrl/$Version/$asset" -OutFile "$tmp\$App.exe"

  Write-Host "Verifying checksum ..."
  $checksums = Invoke-WebRequest -UseBasicParsing -Uri "$BaseUrl/$Version/checksums.txt" | Select-Object -ExpandProperty Content
  $expected = ($checksums -split "`n" | Where-Object { $_ -match " $asset$" } | ForEach-Object { ($_ -split " ")[0] }).Trim()
  $actual = (Get-FileHash -Algorithm SHA256 "$tmp\$App.exe").Hash.ToLower()
  if ($expected -ne $actual) { throw "Checksum mismatch" }

  New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
  Copy-Item "$tmp\$App.exe" -Destination "$BinDir\taskly-mcp.exe" -Force
  Write-Host "Installed to $BinDir\taskly-mcp.exe"
  Write-Host "Add $BinDir to PATH, or run $BinDir\taskly-mcp.exe directly."
}
finally {
  Remove-Item -Recurse -Force $tmp
}
