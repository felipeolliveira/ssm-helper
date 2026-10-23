// Package msg carries what the backend tells the user as codes, never as
// text: the frontend translates each code with its parameters. A raw
// technical detail (such as an AWS error) may come along and is shown as is.
package msg

import (
	"encoding/json"
	"errors"
)

type Message struct {
	Code   string            `json:"code"`
	Params map[string]string `json:"params,omitempty"`
	Detail string            `json:"detail,omitempty"`
}

// Error carries a Message. Error() returns it as JSON, so a Wails binding
// rejects its promise with a string the frontend can parse.
type Error struct {
	Message
	cause error
}

// New builds a message from a code and key/value parameter pairs.
func New(code string, params ...string) Message {
	m := Message{Code: code}
	for i := 0; i+1 < len(params); i += 2 {
		if m.Params == nil {
			m.Params = map[string]string{}
		}
		m.Params[params[i]] = params[i+1]
	}
	return m
}

// Err builds an error from a code and key/value parameter pairs.
func Err(code string, params ...string) *Error {
	return &Error{Message: New(code, params...)}
}

// Wrap builds an error whose detail is the text of err; errors.Is and
// errors.As still reach err.
func Wrap(code string, err error, params ...string) *Error {
	e := Err(code, params...)
	if err != nil {
		e.Detail = err.Error()
		e.cause = err
	}
	return e
}

func (e *Error) Error() string {
	data, err := json.Marshal(e.Message)
	if err != nil {
		return e.Code
	}
	return string(data)
}

func (e *Error) Unwrap() error { return e.cause }

// Is matches another *Error with the same code, so codes work as sentinels.
func (e *Error) Is(target error) bool {
	var t *Error
	return errors.As(target, &t) && t.Code == e.Code && t.cause == nil && len(t.Params) == 0
}

// From turns any error into a Message: the outermost *Error, or
// UnexpectedError with the error text as detail. A nil error gives an empty Message.
func From(err error) Message {
	if err == nil {
		return Message{}
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Message
	}
	return Message{Code: UnexpectedError, Detail: err.Error()}
}
