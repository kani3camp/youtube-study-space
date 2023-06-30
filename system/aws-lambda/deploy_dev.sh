################### これはテスト環境用です ###################


# set_desired_max_seats, rooms_state, youtube_organize_database, daily_organize_database, check_live_stream_status
# lambda_sandbox, transfer_collection_history_bigquery, process_user_rp_parallel


# Windows (PowerShell)
cd system; cd aws-lambda;
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"; aws configure set region us-east-1
go build -o main    set_desired_max_seats.go
C:\Users\momom\go\bin\build-lambda-zip.exe -output main.zip main
aws lambda create-function --function-name     process_user_rp_parallel     --runtime go1.x --zip-file fileb://main.zip --handler main --role arn:aws:iam::652333062396:role/service-role/my-first-golang-lambda-function-role-cb8uw4th --timeout 120 --profile soraride
aws lambda update-function-code --function-name     set_desired_max_seats     --zip-file fileb://main.zip --profile soraride


# Mac OS
cd system; cd aws-lambda;
aws configure set region us-east-1
GOARCH=amd64 GOOS=linux go build -o main    check_live_stream_status/main.go
zip main.zip main

aws lambda create-function --function-name transfer_collection_history_bigquery --runtime go1.x --zip-file fileb://main.zip --handler main --role arn:aws:iam::652333062396:role/service-role/my-first-golang-lambda-function-role-cb8uw4th --timeout 120 --profile soraride

aws lambda update-function-code --function-name   check_live_stream_status   --zip-file fileb://main.zip --profile soraride
