package mix

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type User struct {
	ID    int    `uri:"id"`
	Name  string `json:"name"`
	Age   int    `header:"age"`
	Phone string `query:"phone"`
}

func TestBind(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://localhost/user/1?phone=123", bytes.NewBufferString(`{"name":"test"}`))
	require.NoError(t, err)
	req.Pattern = "/user/{id}"
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Age", "16")

	var u User
	err = Bind(req, &u)
	require.NoError(t, err)
	// JSON body 分支会直接返回，不会继续绑定 uri/query/header。
	assert.Equal(t, 0, u.ID)
	assert.Equal(t, "test", u.Name)
	assert.Equal(t, 0, u.Age)
	assert.Equal(t, "", u.Phone)
}

type User2 struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Phone string `json:"phone"`
	User  User   `json:"user"`
}

func TestBind2(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://localhost/user/1?phone=123", bytes.NewBufferString(`{"name":"test","user":{"id":1}}`))
	require.NoError(t, err)
	req.Pattern = "/user/{id}"
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Age", "16")

	var u1 User2
	err = Bind(req, &u1)
	require.NoError(t, err)
	assert.Equal(t, 0, u1.ID)
	assert.Equal(t, "test", u1.Name)

	req, err = http.NewRequest(http.MethodPost, "http://localhost/user/2?phone=007", bytes.NewBufferString(`{"name":"test2"}`))
	require.NoError(t, err)
	req.Pattern = "/user/{id}"
	req.SetPathValue("id", "2")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Age", "18")

	var u2 User2
	err = Bind(req, &u2)
	require.NoError(t, err)
	assert.Equal(t, 0, u2.ID)
	assert.Equal(t, "test2", u2.Name)
}

func TestBindFromURIQueryHeaderWithoutBody(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://localhost/user/1?phone=123", nil)
	require.NoError(t, err)
	req.Pattern = "/user/{id}"
	req.SetPathValue("id", "1")
	req.Header.Set("Age", "16")

	var u User
	err = Bind(req, &u)
	require.NoError(t, err)
	assert.Equal(t, 1, u.ID)
	assert.Equal(t, 16, u.Age)
	assert.Equal(t, "123", u.Phone)
	assert.Equal(t, "", u.Name)
}
