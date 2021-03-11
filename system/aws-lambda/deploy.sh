# Windows

# rooms_state, youtube_organize_database, reset_daily_total_study_time
#
# test_rooms,

set GOOS=linux
go build -o main constants.go credential.go response.go    youtube_organize_database.go
C:\Users\momom\go\bin\build-lambda-zip.exe -output main.zip main
aws lambda create-function --function-name     rooms_state     --runtime go1.x --zip-file fileb://main.zip --handler main --role arn:aws:iam::652333062396:role/service-role/my-first-golang-lambda-function-role-cb8uw4th --timeout 10
aws lambda update-function-code --function-name     youtube_organize_database     --zip-file fileb://main.zip


# Mac OS

GOOS=linux go build -o main common.go news.go
zip main.zip main

aws lambda create-function --function-name change_user_info --runtime go1.x --zip-file fileb://main.zip --handler main --role arn:aws:iam::652333062396:role/service-role/my-first-golang-lambda-function-role-cb8uw4th --timeout 10

aws lambda update-function-code --function-name news --zip-file fileb://main.zip
