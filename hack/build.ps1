# Native Windows build; use a C compiler and Vulkan headers for CGO_ENABLED=1.
param(
    [Parameter(Position = 0)]
    [ValidateSet("build", "install")]
    [string]$Action = "build",
    [string]$GoBinary = $(if ($env:GOBINARY) { $env:GOBINARY } else { "go" }),
    [string]$BuildTarget = $env:BUILD_TARGET,
    [string]$Version = $env:VERSION
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    $goos = & $GoBinary env GOOS
    if ($LASTEXITCODE -ne 0 -or $goos -ne "windows") {
        throw "hack/build.ps1 requires a Windows Go target; use hack/build.sh on Linux/macOS."
    }
    if (-not $Version) {
        $Version = "dev"
        if (Get-Command git -ErrorAction SilentlyContinue) {
            $gitVersion = & git -C $root describe --tags --always --dirty=-dev 2>$null
            if ($LASTEXITCODE -eq 0) { $Version = $gitVersion }
        }
    }
    if ($Version -notmatch '^[a-zA-Z0-9][a-zA-Z0-9._+-]*$') {
        throw "VERSION must contain only letters, digits, dots, underscores, plus or minus."
    }
    if (-not $BuildTarget) { $BuildTarget = Join-Path $root "bin/taple-tui.exe" }
    $BuildTarget = [IO.Path]::GetFullPath($BuildTarget)
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $BuildTarget) | Out-Null
    if (-not $env:CGO_ENABLED) { $env:CGO_ENABLED = "1" }
    $ldflags = "$env:LDFLAGS -X github.com/go-musicfox/go-musicfox/internal/types.AppVersion=$Version"
    & $GoBinary build -trimpath -buildvcs=false -ldflags $ldflags -o $BuildTarget ./cmd
    if ($LASTEXITCODE -ne 0) { throw "Build failed with exit code $LASTEXITCODE." }

    if ($Action -eq "install") {
        $destination = & $GoBinary env GOBIN
        if ($LASTEXITCODE -ne 0) { throw "Could not read GOBIN." }
        if (-not $destination) {
            $goPath = & $GoBinary env GOPATH
            if ($LASTEXITCODE -ne 0) { throw "Could not read GOPATH." }
            $firstGoPath = ($goPath -split [IO.Path]::PathSeparator)[0]
            $destination = Join-Path $firstGoPath "bin"
        }
        New-Item -ItemType Directory -Force -Path $destination | Out-Null
        Copy-Item -LiteralPath $BuildTarget -Destination (Join-Path $destination "taple-tui.exe")
    }
} finally {
    Pop-Location
}
