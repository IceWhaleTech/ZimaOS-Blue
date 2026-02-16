$content = Get-Content commonfunc.nsh -Encoding Unicode
$newContent = @()
foreach ($line in $content) {
    $newContent += $line
    if ($line -match 'Delete "\$DESKTOP\\$\{PRODUCT_NAME\}\.lnk"') {
        $newContent += "`t# Also try current user context"
        $newContent += "`tSetShellVarContext current"
        $newContent += "`tDelete `"`$DESKTOP\`${PRODUCT_NAME}.lnk`""
        $newContent += "`tSetShellVarContext all"
    }
}
$newContent | Set-Content commonfunc.nsh -Encoding Unicode
Write-Host "Fixed desktop shortcut deletion"
