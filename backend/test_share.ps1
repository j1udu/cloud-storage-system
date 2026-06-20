param(
    [string]$BaseUrl = "http://localhost:8080"
)

$ErrorActionPreference = "Stop"

$Api = "$BaseUrl/api/v1"
$Pass = 0
$Fail = 0
$Total = 0

function Record {
    param(
        [string]$Status,
        [string]$Name
    )

    $script:Total++
    if ($Status -eq "pass") {
        $script:Pass++
        Write-Host "PASS $Name" -ForegroundColor Green
    } else {
        $script:Fail++
        Write-Host "FAIL $Name" -ForegroundColor Red
    }
}

function Assert-Code {
    param(
        [string]$Name,
        [object]$Response,
        [int]$Expected
    )

    if ($Response.code -eq $Expected) {
        Record "pass" $Name
    } else {
        $json = $Response | ConvertTo-Json -Depth 20 -Compress
        Record "fail" "$Name expected code=$Expected got code=$($Response.code) response=$json"
    }
}

function Assert-True {
    param(
        [string]$Name,
        [bool]$Value
    )

    if ($Value) {
        Record "pass" $Name
    } else {
        Record "fail" $Name
    }
}

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Path,
        [object]$Body = $null,
        [string]$Token = ""
    )

    $headers = @{}
    if ($Token -ne "") {
        $headers["Authorization"] = "Bearer $Token"
    }

    $uri = "$Api$Path"
    if ($null -eq $Body) {
        return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers
    }

    $json = $Body | ConvertTo-Json -Compress
    return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers -ContentType "application/json" -Body $json
}

function Upload-File {
    param(
        [string]$FilePath,
        [string]$Token,
        [int]$ParentID = 0
    )

    $responseText = & curl.exe -sS -X POST "$Api/files/upload" `
        -H "Authorization: Bearer $Token" `
        -F "file=@$FilePath" `
        -F "parent_id=$ParentID"
    return $responseText | ConvertFrom-Json
}

Write-Host "Share API test"
Write-Host "Target: $BaseUrl"

$ping = Invoke-RestMethod -Uri "$BaseUrl/ping"
Assert-Code "ping returns code 0" $ping 0

$ts = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$suffix = "$ts$([System.Guid]::NewGuid().ToString('N').Substring(0, 8))"
$password = "test123456"
$userA = "share_a_$suffix"
$userB = "share_b_$suffix"

$res = Invoke-Api "POST" "/auth/register" @{ username = $userA; password = $password }
Assert-Code "register user A" $res 0

$res = Invoke-Api "POST" "/auth/register" @{ username = $userB; password = $password }
Assert-Code "register user B" $res 0

$loginA = Invoke-Api "POST" "/auth/login" @{ username = $userA; password = $password }
Assert-Code "login user A" $loginA 0
$tokenA = $loginA.data.token
Assert-True "login user A returns token" ($tokenA -ne $null -and $tokenA -ne "")

$loginB = Invoke-Api "POST" "/auth/login" @{ username = $userB; password = $password }
Assert-Code "login user B" $loginB 0
$tokenB = $loginB.data.token
Assert-True "login user B returns token" ($tokenB -ne $null -and $tokenB -ne "")

$tmp = New-TemporaryFile
try {
    Set-Content -LiteralPath $tmp.FullName -Value "share test $suffix" -Encoding UTF8

    $upload = Upload-File $tmp.FullName $tokenA 0
    Assert-Code "user A uploads file" $upload 0
    $fileID = [uint64]$upload.data.id
    Assert-True "upload returns file id" ($fileID -gt 0)

    $shareEmpty = Invoke-Api "POST" "/shares" @{ matter_id = $fileID; access_code = ""; expire_hour = 24 } $tokenA
    Assert-Code "user A creates file share without access_code" $shareEmpty 0
    $shareID = [uint64]$shareEmpty.data.id
    $shareToken = [string]$shareEmpty.data.token
    Assert-True "share returns token" ($shareToken -ne "")
    Assert-True "share returns access_code" ($null -ne $shareEmpty.data.access_code)
    Assert-True "share returns expire_at" ($null -ne $shareEmpty.data.expire_at)

    $res = Invoke-Api "DELETE" "/shares/$shareID" $null $tokenB
    Assert-Code "user B cannot cancel user A share" $res 10005

    $list = Invoke-Api "GET" "/shares?page=1&page_size=20" $null $tokenA
    Assert-Code "user A lists own shares" $list 0
    $listed = @($list.data.items | Where-Object { $_.id -eq $shareID }).Count -gt 0
    Assert-True "user A list contains created share" $listed

    $publicInfo = Invoke-Api "GET" "/public/shares/$shareToken"
    Assert-Code "public share info is accessible without login" $publicInfo 0
    Assert-True "public share info returns matter" ($publicInfo.data.matter.id -eq $fileID)

    $download = Invoke-Api "POST" "/public/shares/$shareToken/download" @{ access_code = "" }
    Assert-Code "empty access_code public download succeeds" $download 0
    Assert-True "empty access_code download returns url" ($download.data.url -ne $null -and $download.data.url -ne "")

    $shareCode = Invoke-Api "POST" "/shares" @{ matter_id = $fileID; access_code = "1234"; expire_hour = 24 } $tokenA
    Assert-Code "user A creates file share with access_code" $shareCode 0
    $shareCodeToken = [string]$shareCode.data.token

    $wrongDownload = Invoke-Api "POST" "/public/shares/$shareCodeToken/download" @{ access_code = "wrong" }
    Assert-Code "wrong access_code public download fails" $wrongDownload 10005

    $rightDownload = Invoke-Api "POST" "/public/shares/$shareCodeToken/download" @{ access_code = "1234" }
    Assert-Code "correct access_code public download succeeds" $rightDownload 0
    Assert-True "correct access_code download returns url" ($rightDownload.data.url -ne $null -and $rightDownload.data.url -ne "")

    $deleteFile = Invoke-Api "DELETE" "/files/$fileID" $null $tokenA
    Assert-Code "user A moves shared file to recycle" $deleteFile 0

    $deletedPublicInfo = Invoke-Api "GET" "/public/shares/$shareCodeToken"
    Assert-Code "public info fails after shared file is recycled" $deletedPublicInfo 10005

    $deletedPublicDownload = Invoke-Api "POST" "/public/shares/$shareCodeToken/download" @{ access_code = "1234" }
    Assert-Code "public download fails after shared file is recycled" $deletedPublicDownload 10005

    $cancel = Invoke-Api "DELETE" "/shares/$shareID" $null $tokenA
    Assert-Code "user A cancels own share" $cancel 0

    $canceledInfo = Invoke-Api "GET" "/public/shares/$shareToken"
    Assert-Code "public access fails after cancel" $canceledInfo 10005

    $folderName = "share_folder_$suffix"
    $folder = Invoke-Api "POST" "/folders" @{ parent_id = 0; name = $folderName } $tokenA
    Assert-Code "user A creates folder" $folder 0
    $folderID = [uint64]$folder.data.id

    $neverExpire = Invoke-Api "POST" "/shares" @{ matter_id = $folderID; access_code = ""; expire_hour = 0 } $tokenA
    Assert-Code "expire_hour zero share succeeds" $neverExpire 0
    Assert-True "expire_hour zero returns null expire_at" ($null -eq $neverExpire.data.expire_at)

    $neverExpireToken = [string]$neverExpire.data.token
    $neverExpireInfo = Invoke-Api "GET" "/public/shares/$neverExpireToken"
    Assert-Code "expire_hour zero public info succeeds" $neverExpireInfo 0
} finally {
    if (Test-Path -LiteralPath $tmp.FullName) {
        Remove-Item -LiteralPath $tmp.FullName -Force
    }
}

Write-Host ""
Write-Host "Summary"
Write-Host "Total: $Total"
Write-Host "Passed: $Pass" -ForegroundColor Green
Write-Host "Failed: $Fail" -ForegroundColor Red

if ($Fail -gt 0) {
    exit 1
}
