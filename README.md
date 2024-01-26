# MLHub
MLHUb for CHESS

### API usage
```
# upload ML model
curl http://localhost:port/upload \
    -v -X POST -H "Authorization: bearer $token" \
    -F 'file=@/path/model.tar.gz' \
    -F 'model=model' -F 'type=TensorFlow' -F 'backend=GoFake'
```

```
# get documentation
curl http://localhost:8350/docs/docs
```
