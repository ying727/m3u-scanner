@echo off
SET NAME=m3u-scanner

echo Building for Windows...
SET GOOS=windows
SET GOARCH=amd64
go build -o bin/%NAME%-windows-amd64.exe main.go

echo Building for Linux...
SET GOOS=linux
SET GOARCH=amd64
go build -o bin/%NAME%-linux-amd64 main.go

echo Building for MacOS (Intel)...
SET GOOS=darwin
SET GOARCH=amd64
go build -o bin/%NAME%-darwin-amd64 main.go

echo Building for MacOS (M1/M2/M3)...
SET GOOS=darwin
SET GOARCH=arm64
go build -o bin/%NAME%-darwin-arm64 main.go

echo All builds finished! Check the bin/ folder.
pause