$content = Get-Content commonfunc.nsh -Encoding Unicode
$newContent = @()

foreach ($line in $content) {
    $newContent += $line
    # After Sleep 500, add WriteUninstaller
    if ($line -match 'Sleep 500') {
        $newContent += "`t"
        $newContent += "`t# Create uninstaller"
        $newContent += "`tWriteUninstaller `"`$INSTDIR\uninst.exe`""
    }
}

$newContent | Set-Content commonfunc.nsh -Encoding Unicode
Write-Host "Added WriteUninstaller command"
