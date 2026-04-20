# awsxraymock

Mock the AWS X-Ray API for testing. Can toggle on/off API throttling.

## Build

```sh
go build -o awsxraymockserver -ldflags="-s -w" -v .
```

## Generate TLS certficiates

```sh
openssl req -new -subj "/C=US/ST=Utah/CN=localhost" -newkey rsa:2048 -nodes -keyout localhost.key -out localhost.csr
openssl x509 -req -days 365 -in localhost.csr -signkey localhost.key -out localhost.crt
```

## Run

```sh
./awsxraymockserver -cert localhost.crt -key localhost.key
```

If `-cert` and `-key` are omitted, the server generates a self-signed certificate for `localhost` and `127.0.0.1` at startup.

## Usage

Set the API to normal operation:

```sh
curl -X POST http://localhost:8080/SetOK
```

Set the API to throttle operation, all requests will return 429:

```sh
curl -X POST http://localhost:8080/SetThrottled
```

Otherwise post JSON segment documents at the API endpoint.

```sh
START_TIME=$(date +%s)
HEX_TIME=$(printf '%x\n' $START_TIME)
GUID=$(dd if=/dev/random bs=12 count=1 2>/dev/null | od -An -tx1 | tr -d ' \t\n')
TRACE_ID="1-$HEX_TIME-$GUID"
END_TIME=$(($START_TIME+3))
DOC=$(cat <<EOF
{"trace_id": "$TRACE_ID", "id": "6226467e3f845502", "start_time": $START_TIME.37518, "end_time": $END_TIME.4042, "name": "test.elasticbeanstalk.com"}
EOF
)
export AWS_ACCESS_KEY_ID=7373737373
export AWS_SECRET_ACCESS_KEY=uuigsjdfgiusdf
aws --endpoint-url http://localhost:3000 --no-verify-ssl xray put-trace-segments --trace-segment-documents "$DOC"
```

For the default startup path with an auto-generated certificate:

```sh
aws --endpoint-url https://localhost:8443 --no-verify-ssl xray put-trace-segments --trace-segment-documents "$DOC"
```
