// REST API Documentation
//
// In order to interact with the exchange you need to generate and read/write API KEY
// from the API Management page from your account.
//
// Some of the endpoints are only available with a User Auth Token that is generated by
// the website in the login process.
//
// The API specification is generated from a Swagger 2.0 API Spec file that is available
// as part of our API.
//
// The current version of the API is a prestable version that is subject to change in the
// following period.
//
//	Schemes: http, https
//	Host: paramountdax.com
//	BasePath: /api
//	Version: 0.1.0
//	Contact: API Support<support@paramountdax.com> https://paramountdax.com/contact
//	Terms Of Service: https://paramountdax.com/terms
//
//	Consumes:
//	- application/json
//	- application/x-www-form-urlencoded
//	- multipart/form-data
//
//	Produces:
//	- application/json
//
//	Security:
//	- ApiKey:
//	- UserToken:
//	- AdminToken:
//
//	SecurityDefinitions:
//	ApiKey:
//	     type: apiKey
//	     name: X-Api-Key
//	     in: header
//	     description: An API Key generated by the user from the API Management section of their profile.
//	UserToken:
//	     type: basic
//	     name: Authorisation
//	     in: header
//	     description: A user token generated by a normal login on the platform.
//	AdminToken:
//	     type: basic
//	     name: Authorisation
//	     in: header
//	     description: A token generated by a normal login on the platform by a user with administrative permissions.
//
//swagger:meta
package main // import "gitlab.com/paramountdax-exchange/exchange_api_v2"

import (
	"runtime"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/cmd"
)

func init() {
	// set proc count
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	cmd.Execute()
}