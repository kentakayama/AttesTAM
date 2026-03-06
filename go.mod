module github.com/kentakayama/AttesTAM

go 1.25.3

require (
	github.com/fxamacker/cbor/v2 v2.9.0
	github.com/mattn/go-sqlite3 v1.14.33
	github.com/stretchr/testify v1.11.1
	github.com/veraison/eat v0.0.0-20251105185612-2c0e43e22ea9
	github.com/veraison/go-cose v1.3.1-0.20251008083203-58542e2a46e9
	github.com/veraison/swid v1.1.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.9.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/veraison/eat => github.com/kentakayama/eat v0.0.0-20260212030121-9441b6b7cbb3
	github.com/veraison/go-cose => github.com/kentakayama/go-cose v0.0.0-20260122035816-b936aa60847b
)
