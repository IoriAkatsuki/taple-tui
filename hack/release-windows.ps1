# Build portable Windows release files; no MSI/runtime installation is required.
param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [string]$GoBinary = $(if ($env:GOBINARY) { $env:GOBINARY } else { 'go' })
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    & "$PSScriptRoot/build.ps1" build -Version $Version -GoBinary $GoBinary
    & $GoBinary run ./hack/package -version $Version
    if ($LASTEXITCODE -ne 0) { throw 'Release packaging failed.' }
    $arch = & $GoBinary env GOARCH
    if ($LASTEXITCODE -ne 0) { throw 'Could not determine release architecture.' }
    $name = "taple_${Version}_windows_${arch}"
    $exe = Join-Path $root "dist/$name.exe"
    Copy-Item -LiteralPath (Join-Path $root 'bin/taple.exe') -Destination $exe
    $hash = (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText("$exe.sha256", "$hash  $name.exe`n", [Text.UTF8Encoding]::new($false))
    Write-Host "Release files: $root\dist\$name.*"
} finally {
    Pop-Location
}
