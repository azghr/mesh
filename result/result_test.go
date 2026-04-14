package result

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOk(t *testing.T) {
	r := Ok(42)
	assert.True(t, r.IsOk())
	assert.False(t, r.IsErr())
	assert.Equal(t, 42, r.Unwrap())
	assert.Nil(t, r.UnwrapErr())
}

func TestErr(t *testing.T) {
	err := errors.New("test error")
	r := Err[int](err)
	assert.False(t, r.IsOk())
	assert.True(t, r.IsErr())
	assert.Equal(t, 0, r.UnwrapOr(0))
	assert.Equal(t, err, r.UnwrapErr())
}

func TestUnwrap(t *testing.T) {
	r := Ok("hello")
	assert.Equal(t, "hello", r.Unwrap())
}

func TestUnwrap_Panic(t *testing.T) {
	r := Err[string](errors.New("error"))
	assert.Panics(t, func() {
		r.Unwrap()
	})
}

func TestUnwrapOr(t *testing.T) {
	r := Ok(10)
	assert.Equal(t, 10, r.UnwrapOr(0))

	r = Err[int](errors.New("error"))
	assert.Equal(t, 0, r.UnwrapOr(0))
}

func TestUnwrapOrElse(t *testing.T) {
	r := Ok(10)
	assert.Equal(t, 10, r.UnwrapOrElse(func() int { return 5 }))

	r = Err[int](errors.New("error"))
	assert.Equal(t, 5, r.UnwrapOrElse(func() int { return 5 }))
}

func TestExpect(t *testing.T) {
	r := Ok(10)
	assert.Equal(t, 10, r.Expect("failed"))

	r = Err[int](errors.New("error"))
	assert.Panics(t, func() {
		r.Expect("failed")
	})
}

func TestMap(t *testing.T) {
	r := Ok(2)
	mapped := r.Map(func(v int) int { return v * 2 })
	assert.Equal(t, 4, mapped.Unwrap())

	r = Err[int](errors.New("error"))
	mapped = r.Map(func(v int) int { return v * 2 })
	assert.True(t, mapped.IsErr())
}

func TestMapErr(t *testing.T) {
	r := Err[int](errors.New("original"))
	mapped := r.MapErr(func(e error) error { return errors.New("wrapped: " + e.Error()) })
	assert.Equal(t, "wrapped: original", mapped.UnwrapErr().Error())

	r = Ok(42)
	mapped = r.MapErr(func(e error) error { return e })
	assert.Equal(t, 42, mapped.Unwrap())
}

func TestAnd(t *testing.T) {
	r := Ok(1)
	result := r.And(func() Result[int] { return Ok(2) })
	assert.Equal(t, 2, result.Unwrap())

	r = Err[int](errors.New("error"))
	result = r.And(func() Result[int] { return Ok(2) })
	assert.True(t, result.IsErr())
}

func TestAndThen(t *testing.T) {
	r := Ok(2)
	result := AndThen(r, func(v int) Result[int] { return Ok(v * 3) })
	assert.Equal(t, 6, result.Unwrap())

	r = Err[int](errors.New("error"))
	result = AndThen(r, func(v int) Result[int] { return Ok(v * 3) })
	assert.True(t, result.IsErr())
}

func TestFrom(t *testing.T) {
	r := 42
	var err error = nil
	result := From(r, err)
	assert.Equal(t, 42, result.Unwrap())

	result = From(0, errors.New("error"))
	assert.True(t, result.IsErr())
}

func TestTap(t *testing.T) {
	var tapped bool
	r := Ok(10).Tap(func(v int) { tapped = v == 10 })
	assert.True(t, tapped)
	assert.Equal(t, 10, r.Unwrap())

	tapped = false
	r = Err[int](errors.New("error")).Tap(func(v int) { tapped = true })
	assert.False(t, tapped)
}

func TestTapErr(t *testing.T) {
	var tapped bool
	_ = Err[int](errors.New("error")).TapErr(func(e error) { tapped = true })
	assert.True(t, tapped)

	tapped = false
	_ = Ok(10).TapErr(func(e error) { tapped = true })
	assert.False(t, tapped)
}

func TestIs(t *testing.T) {
	sentinel := errors.New("sentinel")
	r := Err[int](sentinel)
	assert.True(t, r.Is(sentinel))
	assert.False(t, r.Is(errors.New("other")))

	r = Ok(10)
	assert.True(t, r.Is(nil))
}

func TestChained(t *testing.T) {
	// Demonstrate chaining
	result := Ok(5).
		Map(func(v int) int { return v + 1 }).
		Map(func(v int) int { return v * 2 })

	assert.Equal(t, 12, result.Unwrap())
}

func TestWithPointer(t *testing.T) {
	s := "hello"
	r := Ok(&s)
	assert.Equal(t, &s, r.Unwrap())
}

func TestWithSlice(t *testing.T) {
	arr := []int{1, 2, 3}
	r := Ok(arr)
	assert.Equal(t, arr, r.Unwrap())
}
