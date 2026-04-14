// Package result provides a generic Result type for explicit error handling.
//
// This package implements a Result type similar to Rust's, providing a cleaner
// way to handle errors without panics. It encourages explicit error checking
// and provides chainable methods for common patterns.
//
// Example:
//
//	result := result.Ok(42)
//	value, err := result.Unwrap()
//
//	result := result.Err[int](errors.New("failed"))
//	value := result.UnwrapOr(0)
package result

import "errors"

// Result represents either a success value or an error
type Result[T any] struct {
	value T
	err   error
}

// IsOk returns true if the result is a success value
func (r Result[T]) IsOk() bool {
	return r.err == nil
}

// IsErr returns true if the result is an error
func (r Result[T]) IsErr() bool {
	return r.err != nil
}

// Unwrap returns the value or panics if error
func (r Result[T]) Unwrap() T {
	if r.err != nil {
		panic(r.err)
	}
	return r.value
}

// UnwrapOr returns the value or a default value if error
func (r Result[T]) UnwrapOr(def T) T {
	if r.err != nil {
		return def
	}
	return r.value
}

// UnwrapOrElse returns the value or calls the function if error
func (r Result[T]) UnwrapOrElse(fn func() T) T {
	if r.err != nil {
		return fn()
	}
	return r.value
}

// UnwrapErr returns the error or nil
func (r Result[T]) UnwrapErr() error {
	return r.err
}

// Expect panics with a custom message if error
func (r Result[T]) Expect(msg string) T {
	if r.err != nil {
		panic(msg + ": " + r.err.Error())
	}
	return r.value
}

// Ok creates a success result
func Ok[T any](value T) Result[T] {
	return Result[T]{value: value, err: nil}
}

// Err creates an error result
func Err[T any](err error) Result[T] {
	var zero T
	return Result[T]{value: zero, err: err}
}

// Map transforms the success value
func (r Result[T]) Map(fn func(T) T) Result[T] {
	if r.err != nil {
		return r
	}
	return Result[T]{value: fn(r.value), err: nil}
}

// MapErr transforms the error
func (r Result[T]) MapErr(fn func(error) error) Result[T] {
	if r.err == nil {
		return r
	}
	var zero T
	return Result[T]{value: zero, err: fn(r.err)}
}

// And then applies another Result-returning function
func (r Result[T]) And(fn func() Result[T]) Result[T] {
	if r.err != nil {
		return r
	}
	return fn()
}

// AndThen chains a Result-returning function
func AndThen[T, U any](r Result[T], fn func(T) Result[U]) Result[U] {
	if r.err != nil {
		var zero U
		return Result[U]{value: zero, err: r.err}
	}
	return fn(r.value)
}

// From creates a Result from a value and error
func From[T any](value T, err error) Result[T] {
	if err != nil {
		var zero T
		return Result[T]{value: zero, err: err}
	}
	return Result[T]{value: value, err: nil}
}

// Tap executes a function if the result is ok (for logging/_side effects)
func (r Result[T]) Tap(fn func(T)) Result[T] {
	if r.err == nil {
		fn(r.value)
	}
	return r
}

// TapErr executes a function if the result is an error
func (r Result[T]) TapErr(fn func(error)) Result[T] {
	if r.err != nil {
		fn(r.err)
	}
	return r
}

// Is checks if the result error matches a specific error
func (r Result[T]) Is(target error) bool {
	if r.err == nil {
		return target == nil
	}
	return errors.Is(r.err, target)
}
