SET YYYY=%DATE:~0,4%
SET MM=%DATE:~5,2%
SET DD=%DATE:~8,2%

go build -ldflags="-X main.BuildDate=%YYYY%-%MM%-%DD%" -o zapm.exe cmd/zapm.go
