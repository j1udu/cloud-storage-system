param(
    [string]$BaseUrl = "http://localhost:8080"
)

$ErrorActionPreference = "Stop"

$Api = "$BaseUrl/api/v1"
$Token = ""
$Token2 = ""
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

    $json = $Response | ConvertTo-Json -Compress -Depth 10
    Record "fail" "$Name expected code=$Expected got code=$($Response.code) response=$json"
}

function Assert-Items-Has-Id {
    param(
        [string]$Name,
        [object]$Response,
        [uint64]$Id
    )

    $found = @($Response.data.items | Where-Object { [uint64]$_.id -eq $Id }).Count -gt 0
    if ($found) {
        Record "pass" $Name
        return
    }

    Record "fail" $Name
}

function Assert-Items-Missing-Id {
    param(
        [string]$Name,
        [object]$Response,
        [uint64]$Id
    )

    $found = @($Response.data.items | Where-Object { [uint64]$_.id -eq $Id }).Count -gt 0
    if (-not $found) {
        Record "pass" $Name
        return
    }

    Record "fail" $Name
}

Write-Host "Backend recycle API test"
Write-Host "Target: $BaseUrl"

Write-Section "1. Health"
$res = Invoke-WebRequest -Method GET -Uri "$BaseUrl/ping" -UseBasicParsing
$ping = $res.Content | ConvertFrom-Json
Assert-Code "ping returns code 0" $ping 0

Write-Section "2. Auth"
$stamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$suffix = Get-Random -Minimum 100000 -Maximum 999999
$username = "recycle_$stamp`_$suffix"
$username2 = "recycle2_$stamp`_$suffix"
$password = "test123456"

$res = Invoke-Api "POST" "/auth/register" @{ username = $username; password = $password }
Assert-Code "register first user" $res 0

$res = Invoke-Api "POST" "/auth/login" @{ username = $username; password = $password }
Assert-Code "login first user" $res 0
$Token = $res.data.token
if ($Token) {
    Record "pass" "first user token returned"
}
else {
    Record "fail" "first user token returned"
}

$res = Invoke-Api "POST" "/auth/register" @{ username = $username2; password = $password }
Assert-Code "register second user" $res 0

$res = Invoke-Api "POST" "/auth/login" @{ username = $username2; password = $password }
Assert-Code "login second user" $res 0
$Token2 = $res.data.token

Write-Section "3. Create folder tree"
$rootName = "recycle_root_$stamp`_$suffix"
$childName = "recycle_child_$stamp`_$suffix"
$tmpFile = New-TemporaryFile
Set-Content -LiteralPath $tmpFile.FullName -Value "recycle permanent delete $stamp $suffix" -Encoding UTF8

$res = Upload-File $tmpFile.FullName $Token 0
Assert-Code "upload standalone file" $res 0
$standaloneFileID = [uint64]$res.data.id

$res = Invoke-Api "POST" "/folders" @{ parent_id = 0; name = $rootName }
Assert-Code "create root folder" $res 0
$rootID = [uint64]$res.data.id

$res = Upload-File $tmpFile.FullName $Token $rootID
Assert-Code "upload child file" $res 0
$childFileID = [uint64]$res.data.id

$res = Upload-File $tmpFile.FullName $Token $rootID
Assert-Code "upload pre-deleted child file" $res 0
$preDeletedFileID = [uint64]$res.data.id

$res = Invoke-Api "POST" "/folders" @{ parent_id = $rootID; name = $childName }
Assert-Code "create child folder" $res 0
$childID = [uint64]$res.data.id

$res = Invoke-Api "GET" "/files?folder_id=0&page=1&page_size=100"
Assert-Code "list root before delete" $res 0
Assert-Items-Has-Id "root folder visible before delete" $res $rootID

$res = Invoke-Api "GET" "/files?folder_id=$rootID&page=1&page_size=100"
Assert-Code "list child before delete" $res 0
Assert-Items-Has-Id "child folder visible before delete" $res $childID
Assert-Items-Has-Id "child file visible before delete" $res $childFileID
Assert-Items-Has-Id "pre-deleted child file visible before delete" $res $preDeletedFileID

Write-Section "4. Move folder tree to recycle"
$res = Invoke-Api "DELETE" "/files/$preDeletedFileID"
Assert-Code "delete child file before parent delete" $res 0

$res = Invoke-Api "DELETE" "/files/$rootID"
Assert-Code "delete root folder" $res 0

$res = Invoke-Api "GET" "/files?folder_id=0&page=1&page_size=100"
Assert-Code "list root after delete" $res 0
Assert-Items-Missing-Id "deleted root hidden from normal list" $res $rootID

$res = Invoke-Api "GET" "/files?folder_id=$rootID&page=1&page_size=100"
Assert-Code "list deleted root children" $res 0
Assert-Items-Missing-Id "deleted child hidden from normal child list" $res $childID
Assert-Items-Missing-Id "deleted child file hidden from normal child list" $res $childFileID

$res = Invoke-Api "GET" "/recycle?page=1&page_size=100"
Assert-Code "list recycle after delete" $res 0
Assert-Items-Has-Id "deleted root appears in recycle" $res $rootID
Assert-Items-Has-Id "deleted child appears in recycle" $res $childID
Assert-Items-Has-Id "deleted child file appears in recycle" $res $childFileID
Assert-Items-Has-Id "previously deleted child file remains in recycle" $res $preDeletedFileID

Write-Section "5. Restore checks"
$res = Invoke-Api "PUT" "/recycle/$childID/restore"
Assert-Code "restore child with recycled parent fails" $res 10005

$res = Invoke-Api "PUT" "/recycle/$rootID/restore" $null $Token2
Assert-Code "other user cannot restore recycled folder" $res 10005

$res = Invoke-Api "PUT" "/recycle/$rootID/restore"
Assert-Code "restore root folder" $res 0

$res = Invoke-Api "GET" "/files?folder_id=0&page=1&page_size=100"
Assert-Code "list root after restore" $res 0
Assert-Items-Has-Id "restored root visible in normal list" $res $rootID

$res = Invoke-Api "GET" "/files?folder_id=$rootID&page=1&page_size=100"
Assert-Code "list child after restore" $res 0
Assert-Items-Has-Id "restored child visible in normal child list" $res $childID
Assert-Items-Has-Id "restored child file visible in normal child list" $res $childFileID
Assert-Items-Missing-Id "previously deleted child file not restored with parent" $res $preDeletedFileID

$res = Invoke-Api "GET" "/recycle?page=1&page_size=100"
Assert-Code "list recycle after parent restore" $res 0
Assert-Items-Has-Id "previously deleted child file still in recycle after parent restore" $res $preDeletedFileID

Write-Section "6. Permanent delete checks"
$res = Invoke-Api "DELETE" "/files/$standaloneFileID"
Assert-Code "delete standalone file before permanent delete" $res 0

$res = Invoke-Api "DELETE" "/recycle/$standaloneFileID"
Assert-Code "permanently delete standalone file" $res 0

$res = Invoke-Api "PUT" "/recycle/$standaloneFileID/restore"
Assert-Code "restore permanently deleted standalone file fails" $res 10005

$res = Invoke-Api "DELETE" "/files/$rootID"
Assert-Code "delete root before permanent delete" $res 0

$res = Invoke-Api "DELETE" "/recycle/$rootID" $null $Token2
Assert-Code "other user cannot permanently delete recycled folder" $res 10005

$res = Invoke-Api "DELETE" "/recycle/$rootID"
Assert-Code "permanently delete root folder" $res 0

$res = Invoke-Api "GET" "/recycle?page=1&page_size=100"
Assert-Code "list recycle after permanent delete" $res 0
Assert-Items-Missing-Id "permanently deleted root hidden from recycle" $res $rootID
Assert-Items-Missing-Id "permanently deleted child hidden from recycle" $res $childID
Assert-Items-Missing-Id "permanently deleted child file hidden from recycle" $res $childFileID
Assert-Items-Missing-Id "previously deleted child file hidden after parent permanent delete" $res $preDeletedFileID

$res = Invoke-Api "PUT" "/recycle/$rootID/restore"
Assert-Code "restore permanently deleted root fails" $res 10005

$res = Invoke-Api "PUT" "/recycle/$childFileID/restore"
Assert-Code "restore permanently deleted child file fails" $res 10005

$res = Invoke-Api "PUT" "/recycle/$preDeletedFileID/restore"
Assert-Code "restore permanently deleted previous child file fails" $res 10005

if (Test-Path -LiteralPath $tmpFile.FullName) {
    Remove-Item -LiteralPath $tmpFile.FullName -Force
}

Write-Host ""
Write-Host "Summary"
Write-Host "  Total: $Total"
Write-Host "  Passed: $Pass" -ForegroundColor Green
Write-Host "  Failed: $Fail" -ForegroundColor Red

if ($Fail -gt 0) {
    exit 1
}
