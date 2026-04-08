@echo off
setlocal enabledelayedexpansion

set "SIGN_TARGET=%~1"
if "!SIGN_TARGET!"=="" (
    echo [ERROR] No file specified for signing.
    exit /b 1
)
if not exist "!SIGN_TARGET!" (
    echo [ERROR] Signing target not found: !SIGN_TARGET!
    exit /b 1
)

if not defined WINDOWS_CERTIFICATE_THUMBPRINT (
    echo [WARN] WINDOWS_CERTIFICATE_THUMBPRINT not set, skipping Windows code signing.
    exit /b 0
)

set "CERT_THUMBPRINT=!WINDOWS_CERTIFICATE_THUMBPRINT!"
set "CERT_THUMBPRINT=!CERT_THUMBPRINT: =!"
if "!CERT_THUMBPRINT!"=="" (
    echo [WARN] WINDOWS_CERTIFICATE_THUMBPRINT is empty after trimming, skipping Windows code signing.
    exit /b 0
)

if defined WINDOWS_TIMESTAMP_URL (
    set "TIMESTAMP_URL=!WINDOWS_TIMESTAMP_URL!"
) else (
    set "TIMESTAMP_URL=http://timestamp.digicert.com"
)

set "SIGNTOOL_EXE="
if defined SIGNTOOL_PATH set "SIGNTOOL_EXE=!SIGNTOOL_PATH!"
if defined SIGNTOOL_EXE (
    if not exist "!SIGNTOOL_EXE!" (
        echo [ERROR] SIGNTOOL_PATH does not exist: !SIGNTOOL_EXE!
        exit /b 1
    )
) else (
    for /f "delims=" %%I in ('where signtool 2^>nul') do (
        set "SIGNTOOL_EXE=%%I"
        goto :signtool_found
    )
    echo [ERROR] signtool.exe not found. Install Windows SDK or set SIGNTOOL_PATH.
    exit /b 1
)

:signtool_found
echo [INFO] Signing !SIGN_TARGET!...

if defined WINDOWS_SIGN_CSP (
    if defined WINDOWS_SIGN_KC (
        call "!SIGNTOOL_EXE!" sign /sha1 "!CERT_THUMBPRINT!" /fd sha256 /tr "!TIMESTAMP_URL!" /td sha256 /csp "!WINDOWS_SIGN_CSP!" /kc "!WINDOWS_SIGN_KC!" "!SIGN_TARGET!"
    ) else (
        call "!SIGNTOOL_EXE!" sign /sha1 "!CERT_THUMBPRINT!" /fd sha256 /tr "!TIMESTAMP_URL!" /td sha256 /csp "!WINDOWS_SIGN_CSP!" "!SIGN_TARGET!"
    )
) else (
    if defined WINDOWS_SIGN_KC (
        call "!SIGNTOOL_EXE!" sign /sha1 "!CERT_THUMBPRINT!" /fd sha256 /tr "!TIMESTAMP_URL!" /td sha256 /kc "!WINDOWS_SIGN_KC!" "!SIGN_TARGET!"
    ) else (
        call "!SIGNTOOL_EXE!" sign /sha1 "!CERT_THUMBPRINT!" /fd sha256 /tr "!TIMESTAMP_URL!" /td sha256 "!SIGN_TARGET!"
    )
)
if errorlevel 1 (
    echo [ERROR] Failed to sign !SIGN_TARGET!
    exit /b 1
)

call "!SIGNTOOL_EXE!" verify /pa "!SIGN_TARGET!" >nul
if errorlevel 1 (
    echo [ERROR] Signature verification failed for !SIGN_TARGET!
    exit /b 1
)

echo [OK] Signed: !SIGN_TARGET!
exit /b 0
