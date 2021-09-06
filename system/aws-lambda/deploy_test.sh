################### これはテスト環境用です ###################


# rooms_state, youtube_organize_database, reset_daily_total_study_time


# Windows (PowerShell)
set GOARCH=amd64; set GOOS=linux; aws configure set region ap-northeast-3
go build -o main constants.go credential.go response.go    rooms_state.go
C:\Users\momom\go\bin\build-lambda-zip.exe -output main.zip main
aws lambda create-function --function-name     rooms_state     --runtime go1.x --zip-file fileb://main.zip --handler main --role arn:aws:iam::652333062396:role/service-role/my-first-golang-lambda-function-role-cb8uw4th --timeout 10
aws lambda update-function-code --function-name     rooms_state     --zip-file fileb://main.zip


# Mac OS
GOARCH=amd64 && GOOS=linux && aws configure set region ap-northeast-3
go build -o main constants.go credential.go response.go    rooms_state.go
zip main.zip main

aws lambda create-function --function-name     rooms_state     --runtime go1.x --zip-file fileb://main.zip --handler main --role arn:aws:iam::652333062396:role/service-role/my-first-golang-lambda-function-role-cb8uw4th --timeout 10

aws lambda update-function-code --function-name     rooms_state     --zip-file fileb://main.zip
