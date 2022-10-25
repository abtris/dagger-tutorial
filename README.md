# Dagger SDK

- https://docs.dagger.io/sdk/go/959738/get-started
- https://www.youtube.com/watch?v=GgMskf-znh4

```
go get dagger.io/dagger@latest
go mod edit -replace github.com/docker/docker=github.com/docker/docker@v20.10.3-0.20220414164044-61404de7df1a+incompatible
````


```
go build
./multibuild https://github.com/kpenfound/greetings-api.git
```
