package actions

// A list of status codes used inside the application. For more details see: https://httpstatuses.com/

// OK - success
const OK = 200

// Created - resource created
const Created = 201

// BadRequest - sent when a bad request was submitted by the client
const BadRequest = 400

// Unauthorized - when the user did not login before attempting to access the resource
const Unauthorized = 401

// AccessDenied - when the use does not have access to the resource with the given login token
const AccessDenied = 403

// NotFound - the resource identified by the given ID does not exist
const NotFound = 404

// PreconditionFailed - a condition must be met before the request can be processed
const PreconditionFailed = 412

// OTPNotEnabled - OTP must be enabled before the resource can be created
const OTPNotEnabled = PreconditionFailed

// ValidationFailed - the request did not pass field verification
const ValidationFailed = 422

// Locked - the resource/action is locked and it can be unlocked using a Two Factor Auth code
const Locked = 423

// OTPRequired - same as locked
const OTPRequired = Locked

// ServerError - internal server error
const ServerError = 500
