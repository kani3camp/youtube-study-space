################### これはテスト環境用です ###################


# rooms_state, youtube_organize_database, reset_daily_total_study_time, check_live_stream_status, lambda_sandbox


# Windows (PowerShell)
# ディレクトリを移動
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"; aws configure set region us-east-1
go build -o main    lambda_sandbox.go
C:\Users\momom\go\bin\build-lambda-zip.exe -output main.zip main
aws lambda create-function --function-name     lambda_sandbox     --runtime go1.x --zip-file fileb://main.zip --handler main --role arn:aws:iam::652333062396:role/service-role/my-first-golang-lambda-function-role-cb8uw4th --timeout 10
aws lambda update-function-code --function-name     lambda_sandbox     --zip-file fileb://main.zip


# Mac OS
GOARCH=amd64 && GOOS=linux && aws configure set region us-east-1 &&  go build -o main common.go news.go
zip main.zip main

aws lambda create-function --function-name change_user_info --runtime go1.x --zip-file fileb://main.zip --handler main --role arn:aws:iam::652333062396:role/service-role/my-first-golang-lambda-function-role-cb8uw4th --timeout 10

aws lambda update-function-code --function-name news --zip-file fileb://main.zip
