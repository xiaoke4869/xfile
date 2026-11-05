@echo off
setlocal
node "%~dp0package.mjs" %*
exit /b %errorlevel%
