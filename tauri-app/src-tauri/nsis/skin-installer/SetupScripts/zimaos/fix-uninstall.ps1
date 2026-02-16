$content = Get-Content commonfunc.nsh -Encoding Unicode
$newContent = @()

foreach ($line in $content) {
    $newContent += $line
    # After deleting desktop shortcut in 'all' context, also try 'current'
    if ($line -match 'Delete "\$DESKTOP') {
        $newContent += "`t# Also try current user context"
        $newContent += "`tSetShellVarContext current"
        $newContent += "`tDelete `"`$DESKTOP\`${PRODUCT_NAME}.lnk`""
        $newContent += "`tSetShellVarContext all"
    }
}

$newContent | Set-Content commonfunc.nsh -Encoding Unicode
Write-Host "Fixed: Desktop shortcut will be deleted from both contexts"
