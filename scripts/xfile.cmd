@echo off
setlocal
cd /d "%~dp0.."
set "PATH=%CD%\.tools;%PATH%"
set "GOMODCACHE=%CD%\.cache\gomod"
set "GOCACHE=%CD%\.cache\go-build"
if "%~1"=="" (wails3 dev) else (wails3 %*)
