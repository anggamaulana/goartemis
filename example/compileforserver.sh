rm artemis_test
GOOS=linux GOARCH=amd64 go build -v main.go
mv main artemis_test