@echo off
setlocal enabledelayedexpansion

rem Wrapper for cl.exe that filters out GCC-specific flags
rem This allows Go CGO to work with MSVC

set "ARGS="

:parse_args
if "%~1"=="" goto run_cl

rem Skip GCC-specific flags that MSVC doesn't support
if "%~1"=="-dM" (
    shift
    goto parse_args
)

rem Skip all -W flags (warnings and linker flags) - check first char after dash
set "arg=%~1"
if "!arg:~0,2!"=="-W" (
    shift
    goto parse_args
)

rem Skip all -f flags (feature flags) - check first char after dash
if "!arg:~0,2!"=="-f" (
    shift
    goto parse_args
)

rem Skip -m flags (machine/arch flags)
if "!arg:~0,2!"=="-m" (
    shift
    goto parse_args
)

rem Skip -g flags (debug info)
if "!arg:~0,2!"=="-g" (
    shift
    goto parse_args
)

rem Skip -pthread
if "!arg!"=="-pthread" (
    shift
    goto parse_args
)

rem Convert -I to /I for include paths
if "%~1"=="-I" (
    set "ARGS=!ARGS! /I%~2"
    shift
    shift
    goto parse_args
)

rem Handle -I with path attached (e.g., -I/path)
if "%~1:~0,2%"=="-I" (
    set "path=%~1:~2%"
    set "ARGS=!ARGS! /I!path!"
    shift
    goto parse_args
)

rem Convert -D to /D for defines
if "%~1"=="-D" (
    set "ARGS=!ARGS! /D%~2"
    shift
    shift
    goto parse_args
)

rem Handle -D with define attached (e.g., -DFOO)
if "%~1:~0,2%"=="-D" (
    set "define=%~1:~2%"
    set "ARGS=!ARGS! /D!define!"
    shift
    goto parse_args
)

rem Convert -O flags to /O
if "%~1"=="-O2" (
    set "ARGS=!ARGS! /O2"
    shift
    goto parse_args
)
if "%~1"=="-O3" (
    set "ARGS=!ARGS! /O2"
    shift
    goto parse_args
)

rem Convert -o to /Fo for output
if "%~1"=="-o" (
    set "ARGS=!ARGS! /Fo%~2"
    shift
    shift
    goto parse_args
)

rem Convert -c (compile only) to /c
if "%~1"=="-c" (
    set "ARGS=!ARGS! /c"
    shift
    goto parse_args
)

rem Pass through everything else
set "ARGS=!ARGS! %~1"
shift
goto parse_args

:run_cl
cl.exe /nologo !ARGS!
exit /b %ERRORLEVEL%
