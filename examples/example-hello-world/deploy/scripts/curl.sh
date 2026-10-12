# create
curl --request POST \
  --url http://localhost:80/helloworld/api/v1/CreateModule \
  --header 'Content-Type: application/json' \
  --data '{
	"name": "example_001",
	"description": "module"
}'

# list
curl --request POST \
  --url http://localhost:80/helloworld/api/v1/ListModule

# get
curl --request POST \
  --url http://localhost:80/helloworld/api/v1/GetModule \
  --data '{
	"id": "1"
}'

# put
curl --request POST \
  --url http://localhost:80/helloworld/api/v1/UpdateModule \
  --header 'Content-Type: application/json' \
  --data '{
  "id": "1",
	"description": "update module"
}'

# delete
curl --request POST \
  --url http://localhost:80/helloworld/api/v1/DeleteModule \
  --data '{
	"id": "1"
}'