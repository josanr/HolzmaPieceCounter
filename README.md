# HolzmaPieceCounter
Service to monitor piece execution by Homag Holzma CutMatic v4.


Build executable:
//i build for 386 because we have one tool which uses Windows XP as OS
GOOS=windows GOARCH=386 go build -o out/piece_counter.exe main.go