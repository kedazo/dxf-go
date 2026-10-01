#!/usr/bin/pwsh

Set-StrictMode -version 2.0
$ErrorActionPreference = "Stop"

function Fail([string]$message) {
    throw $message
}

try {
    go version || Fail "Error reporting `go` version"
    go generate || Fail "Error generating code"
    git diff --exit-code -- '*.generated.go' || Fail "Generated code is out of date; run ``go generate`` and commit the result"
    go build -v || Fail "Error building library"
    go test -v || Fail "Error testing library"
}
catch {
    Write-Host $_
    Write-Host $_.Exception
    Write-Host $_.ScriptStackTrace
    exit 1
}
