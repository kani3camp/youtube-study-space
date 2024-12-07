FROM golang:1.22 as build
WORKDIR /var/task
COPY . .
ENV GOCACHE=/root/.cache/go-build

RUN --mount=type=cache,target="/root/.cache/go-build" go build -tags lambda.norpc -o set_desired_max_seats aws-lambda/set_desired_max_seats/main.go
RUN --mount=type=cache,target="/root/.cache/go-build" go build -tags lambda.norpc -o youtube_organize_database aws-lambda/youtube_organize_database/main.go
RUN --mount=type=cache,target="/root/.cache/go-build" go build -tags lambda.norpc -o daily_organize_database aws-lambda/daily_organize_database/main.go
RUN --mount=type=cache,target="/root/.cache/go-build" go build -tags lambda.norpc -o check_live_stream_status aws-lambda/check_live_stream_status/main.go
RUN --mount=type=cache,target="/root/.cache/go-build" go build -tags lambda.norpc -o transfer_collection_history_bigquery aws-lambda/transfer_collection_history_bigquery/main.go
RUN --mount=type=cache,target="/root/.cache/go-build" go build -tags lambda.norpc -o process_user_rp_parallel aws-lambda/process_user_rp_parallel/main.go

# Copy artifacts to a clean image
FROM public.ecr.aws/lambda/provided:al2023
WORKDIR /app

COPY --from=build /var/task/set_desired_max_seats ./set_desired_max_seats
COPY --from=build /var/task/youtube_organize_database ./youtube_organize_database
COPY --from=build /var/task/daily_organize_database ./daily_organize_database
COPY --from=build /var/task/check_live_stream_status ./check_live_stream_status
COPY --from=build /var/task/transfer_collection_history_bigquery ./transfer_collection_history_bigquery
COPY --from=build /var/task/process_user_rp_parallel ./process_user_rp_parallel
