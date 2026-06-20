param(
    [string]$BaseUrl = "http://localhost:8080",
    [int64]$ExpectedQuotaBytes = 4096
)

$ErrorActionPreference = "Stop"

$Api = "$BaseUrl/api/v1"
$Token = ""
$Total = 0
$Pass = 0
$Fail = 0

function Write-Section {
    param([string]$Name)
    Write-Host ""
    Write-Host "== $Name =="
}

function Record {
    param(
        [string]$Status,
        [string]$Name
    )

    $script:Total++
    if ($Status -eq "pass") {
        $script:Pass++
        Write-Host "  PASS $Name" -ForegroundColor Green
        return
    }

    $script:Fail++
    Write-Host "  FAIL $Name" -ForegroundColor Red
}

function Assert-Code {
    param(
        [string]$Name,
        [object]$Response,
        [int]$Expected
    )

    if ($Response.code -eq $Expected) {
        Record "pass" $Name
        return
    }

    $json = $Response | ConvertTo-Json -Compress -Depth 20
    Record "fail" "$Name expected code=$Expected got code=$($Response.code) response=$json"
}

function Assert-True {
    param(
        [string]$Name,
        [bool]$Value
    )

    if ($Value) {
        Record "pass" $Name
        return
    }

    Record "fail" $Name
}

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Path,
        [object]$Body = $null,
        [string]$AuthToken = $script:Token
    )

    $headers = @{}
    if ($AuthToken) {
        $headers["Authorization"] = "Bearer $AuthToken"
    }

    $params = @{
        Method = $Method
        Uri = "$Api$Path"
        Headers = $headers
        UseBasicParsing = $true
    }

    if ($null -ne $Body) {
        $params["ContentType"] = "application/json"
        $params["Body"] = ($Body | ConvertTo-Json -Compress)
    }

    try {
        $response = Invoke-WebRequest @params
        return $response.Content | ConvertFrom-Json
    }
    catch {
        if ($_.Exception.Response -and $_.Exception.Response.GetResponseStream()) {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $content = $reader.ReadToEnd()
            if ($content) {
                return $content | ConvertFrom-Json
            }
        }
        throw
    }
}

function Upload-File {
    param(
        [string]$FilePath,
        [string]$AuthToken = $script:Token,
        [uint64]$ParentID = 0
    )

    $responseText = & curl.exe -sS -X POST "$Api/files/upload" `
        -H "Authorization: Bearer $AuthToken" `
        -F "file=@$FilePath" `
        -F "parent_id=$ParentID"
    return $responseText | ConvertFrom-Json
}

function New-TestFile {
    param(
        [string]$Path,
        [int]$Size
    )

    $bytes = New-Object byte[] $Size
    for ($i = 0; $i -lt $Size; $i++) {
        $bytes[$i] = [byte](($i + 31) % 251)
    }
    [System.IO.File]::WriteAllBytes($Path, $bytes)
}

Write-Host "Storage quota API test"
Write-Host "Target: $BaseUrl"
Write-Host "Expected quota.default_bytes: $ExpectedQuotaBytes"
Write-Host "Start backend with CLOUD_QUOTA_DEFAULT_BYTES=$ExpectedQuotaBytes for this script."

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) "cloud_quota_$([Guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Path $tempDir | Out-Null

try {
    Write-Section "1. Health"
    $pingResponse = Invoke-WebRequest -Method GET -Uri "$BaseUrl/ping" -UseBasicParsing
    $ping = $pingResponse.Content | ConvertFrom-Json
    Assert-Code "ping returns code 0" $ping 0

    Write-Section "2. Auth"
    $stamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
    $suffix = Get-Random -Minimum 100000 -Maximum 999999
    $username = "quota_$stamp`_$suffix"
    $password = "test123456"

    $res = Invoke-Api "POST" "/auth/register" @{ username = $username; password = $password }
    Assert-Code "register user" $res 0

    $res = Invoke-Api "POST" "/auth/login" @{ username = $username; password = $password }
    Assert-Code "login user" $res 0
    $Token = $res.data.token
    Assert-True "login returns token" ($Token -ne $null -and $Token -ne "")

    Write-Section "3. Quota before upload"
    $quota = Invoke-Api "GET" "/storage/quota"
    Assert-Code "quota returns code 0" $quota 0
    $initialUsed = [int64]$quota.data.used_bytes
    Assert-True "initial used_bytes is non-negative" ($initialUsed -ge 0)
    Assert-True "quota_bytes equals configured default_bytes" ([int64]$quota.data.quota_bytes -eq $ExpectedQuotaBytes)
    Assert-True "available_bytes equals quota minus used" ([int64]$quota.data.available_bytes -eq ($ExpectedQuotaBytes - $initialUsed))

    Write-Section "4. Upload increases usage"
    $smallFile = Join-Path $tempDir "quota_small.bin"
    New-TestFile $smallFile 512

    $res = Upload-File $smallFile
    Assert-Code "upload small file" $res 0
    $fileID = [uint64]$res.data.id

    $quotaAfterUpload = Invoke-Api "GET" "/storage/quota"
    Assert-Code "quota after upload returns code 0" $quotaAfterUpload 0
    $usedAfterUpload = [int64]$quotaAfterUpload.data.used_bytes
    Assert-True "used_bytes increases after upload" ($usedAfterUpload -eq ($initialUsed + 512))

    Write-Section "5. Recycle keeps usage"
    $res = Invoke-Api "DELETE" "/files/$fileID"
    Assert-Code "delete file to recycle" $res 0

    $quotaAfterRecycle = Invoke-Api "GET" "/storage/quota"
    Assert-Code "quota after recycle returns code 0" $quotaAfterRecycle 0
    Assert-True "used_bytes does not decrease in recycle" ([int64]$quotaAfterRecycle.data.used_bytes -eq $usedAfterUpload)

    Write-Section "6. Permanent delete releases usage"
    $res = Invoke-Api "DELETE" "/recycle/$fileID"
    Assert-Code "permanent delete file" $res 0

    $quotaAfterPermanentDelete = Invoke-Api "GET" "/storage/quota"
    Assert-Code "quota after permanent delete returns code 0" $quotaAfterPermanentDelete 0
    Assert-True "used_bytes decreases after permanent delete" ([int64]$quotaAfterPermanentDelete.data.used_bytes -eq $initialUsed)

    Write-Section "7. Over quota upload fails"
    $largeFile = Join-Path $tempDir "quota_large.bin"
    New-TestFile $largeFile ([int]($ExpectedQuotaBytes + 1))

    $res = Upload-File $largeFile
    Assert-Code "upload over remaining quota fails" $res 10005
} finally {
    if (Test-Path -LiteralPath $tempDir) {
        Remove-Item -LiteralPath $tempDir -Recurse -Force
    }
}

Write-Host ""
Write-Host "Summary"
Write-Host "  Total: $Total"
Write-Host "  Passed: $Pass" -ForegroundColor Green
Write-Host "  Failed: $Fail" -ForegroundColor Red

if ($Fail -gt 0) {
    exit 1
}
