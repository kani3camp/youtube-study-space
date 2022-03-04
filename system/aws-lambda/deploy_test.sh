################### これはテスト環境用です ###################


# set_desired_max_seats, rooms_state, youtube_organize_database, reset_daily_total_study_time, check_live_stream_status, lambda_sandbox


# Windows (PowerShell)
cd system; cd aws-lambda  # ディレクトリを移動
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"; aws configure set region us-east-1
go build -o main    rooms_state.go
C:\Users\momom\go\bin\build-lambda-zip.exe -output main.zip main
aws lambda create-function --function-name     rooms_state     --runtime go1.x --zip-file fileb://main.zip --handler \\
main --role arn:aws:iam::652333062396:role/service-role/my-first-golang-lambda-function-role-cb8uw4th --timeout 20 --profile soraride
aws lambda update-function-code --function-name     rooms_state     --zip-file fileb://main.zip --profile soraride


# Mac OS
cd system; cd aws-lambda;  # ディレクトリを移動
GOARCH=amd64 GOOS=linux && aws configure set region us-east-1
go build -o main    reset_daily_total_study_time.go
zip main.zip main

aws lambda create-function --function-name change_user_info --runtime go1.x --zip-file fileb://main.zip --handler
main --role arn:aws:iam::652333062396:role/service-role/my-first-golang-lambda-function-role-cb8uw4th --timeout 20 --profile soraride

aws lambda update-function-code --function-name   reset_daily_total_study_time   --zip-file fileb://main.zip --profile soraride
