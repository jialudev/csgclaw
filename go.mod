module csgclaw

go 1.26.2

require github.com/RussellLuo/boxlite/sdks/go v0.7.6

require github.com/larksuite/oapi-sdk-go/v3 v3.5.3

require golang.org/x/term v0.20.0

require golang.org/x/sys v0.20.0 // indirect

replace github.com/RussellLuo/boxlite/sdks/go => ./third_party/boxlite-go
