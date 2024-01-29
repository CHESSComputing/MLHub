# MLHub
MLHUb for CHESS

### API usage
```
# upload ML model
curl http://localhost:port/upload \
    -v -X POST \
    -H "Authorization: bearer $token" \
    -F 'file=@/path/model.tar.gz' \
    -F 'model=model' -F 'type=TensorFlow' -F 'backend=GoFake'

# list current models
curl http://localhost:port/models

# download specific model
curl http://localhost:port/models/<model_name>

# predict results for given model for provided file.json input
curl http://localhost:port/predict \
    -v -X POST \
    -H "Authorization: bearer $token" \
    -H "Content-type: application/json" \
    -d@/path/file.json

# delete existing model
curl http://localhost:port/models/<model_name> \
    -v -X DELETE \
    -H "Authorization: bearer $token"

# get documentation
curl http://localhost:port/docs/docs
```
